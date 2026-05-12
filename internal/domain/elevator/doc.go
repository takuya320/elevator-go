// Package elevator はエレベーターシミュレータのコアドメイン。
//
// HTTP・永続化・wall clock・ID 生成への依存を持たない。
// それらは usecase 層が抽象化し、infrastructure 層が実装する。
//
// 並行性: ElevatorBank 集約はスレッドセーフではない。
// 直列化は呼び出し側 (usecase.Locker) の責務。
package elevator
