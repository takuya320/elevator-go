// Package usecase はドメインと infrastructure を協調させるアプリケーションサービス。
//
// 各 UseCase は次の流れに従う:
//  1. Locker.Lock（全 UseCase で共有し、tick とリクエストの interleave を防ぐ）
//  2. Repository.Find で集約を取得
//  3. 入力 DTO → ドメイン VO に変換
//  4. ドメインのメソッドを呼び出す
//  5. Repository.Save で永続化
//  6. ドメイン → 出力 DTO に変換
//
// Clock / IDGenerator / Locker / SimulationClock を Port として宣言し、
// 実装は infrastructure 層に置く。テストは差し替え可能。
package usecase
