
## Introduction

Microsmith generates random (but valid) Go programs that can be used
to stress-test Go compilers.

### Bugs found

#### gc

- [#23504: internal compiler error: panic during layout](https://github.com/golang/go/issues/23504)
- [#23889: panic: branch too far on arm64](https://github.com/golang/go/issues/23889)
- [#25006: compiler hangs on tiny program](https://github.com/golang/go/issues/25006)
- [#25269: compiler crashes with "stuck in spanz loop" error on s390x with -N](https://github.com/golang/go/issues/25269)
- [#25526: internal compiler error: expected branch at write barrier block](https://github.com/golang/go/issues/25516)
- [#25741: internal compiler error: not lowered: v15, OffPtr PTR64 PTR64](https://github.com/golang/go/issues/25741)
- [#25993: internal compiler error: bad AuxInt value with ssacheck enabled](https://github.com/golang/go/issues/25993)


