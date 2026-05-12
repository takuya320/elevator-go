package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"elevator-go/internal/infrastructure/clock"
	"elevator-go/internal/infrastructure/id"
	"elevator-go/internal/infrastructure/persistence/memory"
	syncpkg "elevator-go/internal/infrastructure/sync"
	"elevator-go/internal/interface/http/server"
	"elevator-go/internal/usecase"
)

func main() {
	addr := envOr("ADDR", ":8080")
	tickInterval := envDuration("TICK_INTERVAL_MS", 1000) * time.Millisecond

	logger := newLogger()
	slog.SetDefault(logger)

	defaultRange, defaultElevators := buildInitialDefaults()

	repo := memory.NewElevatorBankRepository()
	simClk := memory.NewSimulationClock()
	locker := syncpkg.NewMutexLocker()
	wallClock := clock.System{}
	idGen := id.UUID{}

	advance := usecase.NewAdvanceTick(repo, simClk, locker, wallClock)
	getState := usecase.NewGetState(repo, simClk, locker)

	deps := server.HandlerDeps{
		PressHallButton:     usecase.NewPressHallButton(repo, locker, wallClock, idGen),
		PressCarButton:      usecase.NewPressCarButton(repo, locker),
		AdvanceTick:         advance,
		GetVisibleElevators: usecase.NewGetVisibleElevators(repo, locker),
		ResetSimulation:     usecase.NewResetSimulation(repo, simClk, locker, defaultRange, defaultElevators),
		PatchElevator:       usecase.NewPatchElevator(repo, locker),
		CancelHallCall:      usecase.NewCancelHallCall(repo, locker),
		OpenDoor:            usecase.NewOpenDoor(repo, locker),
		CloseDoor:           usecase.NewCloseDoor(repo, locker),
	}

	// 初回リクエストまでに bank を初期化しておく。POST /simulation/reset と同じ経路を使う。
	// 空 input なので usecase に注入した defaults が適用される（UI からの reset も同じ経路）。
	if _, err := deps.ResetSimulation.Execute(context.Background(), usecase.ResetSimulationInput{}); err != nil {
		slog.Error("seed reset failed", "err", err)
		os.Exit(1)
	}

	broadcaster := server.NewBroadcaster()
	sse := server.NewSSEHandler(broadcaster, getState)
	ticker := server.NewAutoTicker(tickInterval, advance, broadcaster)

	router, err := server.NewRouter(server.NewHandler(deps), sse)
	if err != nil {
		slog.Error("init router failed", "err", err)
		os.Exit(1)
	}

	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go ticker.Run(rootCtx)

	srv := &http.Server{Addr: addr, Handler: router}
	go func() {
		slog.Info("listening",
			"addr", addr,
			"ui", "http://localhost"+addr+"/",
			"swagger", "http://localhost"+addr+"/docs",
			"sse", "http://localhost"+addr+"/events",
			"tickInterval", tickInterval,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-rootCtx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("shutdown", "err", err)
	}
}

// LOG_FORMAT=json で構造化ログ、それ以外は人が読みやすいテキスト形式。
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_DEBUG") != "" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	if os.Getenv("LOG_FORMAT") == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// 起動時の階数・台数を env から組み立てる。これが usecase.ResetSimulation の defaults になり、
// /simulation/reset を空 body で呼ぶ UI ボタンや初回 seed でも同じ値が適用される。
// 明らかに壊れた入力（min>=max、count<1）のみ警告してデフォルトに戻す。
func buildInitialDefaults() (usecase.FloorRangeSnapshot, []usecase.ElevatorInit) {
	floorMin := envInt("FLOOR_MIN", 1)
	floorMax := envInt("FLOOR_MAX", 10)
	if floorMin >= floorMax {
		slog.Warn("FLOOR_MIN/FLOOR_MAX invalid, using defaults", "min", floorMin, "max", floorMax)
		floorMin, floorMax = 1, 10
	}
	count := envInt("ELEVATOR_COUNT", 2)
	if count < 1 {
		slog.Warn("ELEVATOR_COUNT must be >= 1, using default", "value", count)
		count = 2
	}
	return usecase.FloorRangeSnapshot{Min: floorMin, Max: floorMax}, initialElevators(count, floorMin, floorMax)
}

// N 台のエレベーターを min〜max に等間隔配置する。1 台なら min、2 台なら両端、
// 3 台以上は等間隔（端点を含む）。
func initialElevators(count, min, max int) []usecase.ElevatorInit {
	out := make([]usecase.ElevatorInit, count)
	for i := 0; i < count; i++ {
		floor := min
		if count > 1 {
			floor = min + (max-min)*i/(count-1)
		}
		out[i] = usecase.ElevatorInit{ID: fmt.Sprintf("ev-%d", i+1), InitialFloor: floor}
	}
	return out
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid env, using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
}

func envDuration(key string, defMs int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(defMs)
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Warn("invalid env, using default", "key", key, "value", v, "defaultMs", defMs)
		return time.Duration(defMs)
	}
	return time.Duration(n)
}
