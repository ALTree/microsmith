
microsmith generates random (but always valid) Go programs that can be
used to stress-test Go compilers.

#### Bugs found

##### gc compiler crashes (39)

- [#23504 internal compiler error: panic during layout](https://github.com/golang/go/issues/23504)
- [#23889 panic: branch too far on arm64](https://github.com/golang/go/issues/23889)
- [#25006 compiler hangs on tiny program](https://github.com/golang/go/issues/25006)
- [#25269 compiler crashes with "stuck in spanz loop" error on s390x with -N](https://github.com/golang/go/issues/25269)
- [#25526 internal compiler error: expected branch at write barrier block](https://github.com/golang/go/issues/25516)
- [#25741 internal compiler error: not lowered: v15, OffPtr PTR64 PTR64](https://github.com/golang/go/issues/25741)
- [#25993 internal compiler error: bad AuxInt value with ssacheck enabled](https://github.com/golang/go/issues/25993)
- [#26024 internal compiler error: expected successors of write barrier block to be plain](https://github.com/golang/go/issues/26024)
- [#26043 internal compiler error: panic during prove](https://github.com/golang/go/issues/26043)
- [#28055 compiler crashes with "VARDEF is not a top level statement" error](https://github.com/golang/go/issues/28055)
- [#29215 internal compiler error: panic during lower](https://github.com/golang/go/issues/29215)
- [#29218 internal compiler error: bad live variable at entry](https://github.com/golang/go/issues/29218)
- [#30257 internal compiler error: panic during lower II](https://github.com/golang/go/issues/30257)
- [#30679 internal compiler error: panic during lower (unreachable)](https://github.com/golang/go/issues/30679)
- [#31915 internal compiler error: val is in reg but not live](https://github.com/golang/go/issues/31915)
- [#32454 internal compiler error: bad live variable at entry II](https://github.com/golang/go/issues/32454)
- [#33903 internal compiler error: panic during short circuit](https://github.com/golang/go/issues/33903)
- [#34520 panic: First time we've seen filename](https://github.com/golang/go/issues/34520)
- [#35157 internal compiler error: aliasing constant which is not registered](https://github.com/golang/go/issues/35157)
- [#35695 panic: Assigning a bogus line to XPos with no file](https://github.com/golang/go/issues/35695)
- [#38359 internal compiler error: can't encode a NaN in AuxInt field](https://github.com/golang/go/issues/38359)
- [#38916 internal compiler error: schedule does not include all values](https://github.com/golang/go/issues/38916)
- [#38946 panic: log2 of 0 on arm64](https://github.com/golang/go/issues/38946)
- [#39472 internal compiler error: schedule does not include all values in block](https://github.com/golang/go/issues/39472)
- [#39505 internal compiler error: Flag* ops should never make it to codegen](https://github.com/golang/go/issues/39505)
- [#42587 illegal combination SRA ADDCON REG REG on mips](https://github.com/golang/go/issues/42587)
- [#42610 internal compiler error: PPC64 shift arg mb out of range](https://github.com/golang/go/issues/42610)
- [#43099 internal compiler error: panic during lower (integer divide by zero)](https://github.com/golang/go/issues/43099)
- [#43701 panic: invalid memory address or nil pointer dereference](https://github.com/golang/go/issues/43701)
- [#44465 internal compiler error: arg 1 does not dominate](https://github.com/golang/go/issues/44465)
- [#45359 internal compiler error: FlagConstant op should never make it to codegen](https://github.com/golang/go/issues/45359)
- [#45589 internal compiler error: Invalid PPC64 rotate mask](https://github.com/golang/go/issues/45589)
- [#45693 internal compiler error: bad live variable at entry](https://github.com/golang/go/issues/45693)
- [#47684 internal compiler error: misuse of CaptureName](https://github.com/golang/go/issues/47684)
- [#47723 internal compiler error: .dict already has a location](https://github.com/golang/go/issues/47723)
- [#48026 internal compiler error: arg does not dominate](https://github.com/golang/go/issues/48026)
- [#48191 data race in compiler building generic program](https://github.com/golang/go/issues/48191)
- [#49122 internal compiler error: can't find source for MOVBload](https://github.com/golang/go/issues/49122)
- [#49242 internal compiler error: found illegal assignment](https://github.com/golang/go/issues/49242)

##### gccgo (2)

- [#44383 internal compiler error: Segmentation fault during GIMPLE pass ealias](https://github.com/golang/go/issues/44383)
- [#47130 internal compiler error: in write_equal_function](https://github.com/golang/go/issues/47130)

##### tinygo (1)

- [#1653 LLVM ERROR: Cannot select: intrinsic %llvm.coro.size](https://github.com/tinygo-org/tinygo/issues/1653)
