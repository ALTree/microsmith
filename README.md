
## Introduction

Microsmith generates random Go programs that can be used to
stress-test Go compilers.

### Bugs found

#### gc compiler crashes

- [#23504: internal compiler error: panic during layout](https://github.com/golang/go/issues/23504)
- [#23889: panic: branch too far on arm64](https://github.com/golang/go/issues/23889)
- [#25006: compiler hangs on tiny program](https://github.com/golang/go/issues/25006)
- [#25269: compiler crashes with "stuck in spanz loop" error on s390x with -N](https://github.com/golang/go/issues/25269)
- [#25526: internal compiler error: expected branch at write barrier block](https://github.com/golang/go/issues/25516)
- [#25741: internal compiler error: not lowered: v15, OffPtr PTR64 PTR64](https://github.com/golang/go/issues/25741)
- [#25993: internal compiler error: bad AuxInt value with ssacheck enabled](https://github.com/golang/go/issues/25993)
- [#26024: internal compiler error: expected successors of write barrier block to be plain](https://github.com/golang/go/issues/26024)
- [#26043: internal compiler error: panic during prove](https://github.com/golang/go/issues/26043)
- [#28055: compiler crashes with "VARDEF is not a top level statement" error](https://github.com/golang/go/issues/28055)
- [#29215: internal compiler error: panic during lower](https://github.com/golang/go/issues/29215)
- [#29218: internal compiler error: bad live variable at entry](https://github.com/golang/go/issues/29218)
- [#30257: internal compiler error: panic during lower II](https://github.com/golang/go/issues/30257)
- [#30679: internal compiler error: panic during lower (unreachable)](https://github.com/golang/go/issues/30679)
- [#31915: internal compiler error: val is in reg but not live](https://github.com/golang/go/issues/31915)
- [#32454: internal compiler error: bad live variable at entry II](https://github.com/golang/go/issues/32454)
- [#33903: internal compiler error: panic during short circuit](https://github.com/golang/go/issues/33903)
- [#34520: panic: First time we've seen filename](https://github.com/golang/go/issues/34520)
