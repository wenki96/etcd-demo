# 流程说明

## 整体说明

demo中实现了基于ETCD的分布式存储和分布式锁功能，满足两种不同粒度（单key/全局）下的数据读取和写入功能，并通过分布式锁保证其互斥性。

程序共包含三种函数：reader，writer和 reseter。其中 reader 负责从 ETCD 中根据 key 读取对应的 value，由get 实现；writer 负责向 ETCD 中加入或更新某个 key/value 键值对，由 put 实现；reseter 负责重置 ETCD 中的所有键值对，由 delete 前缀值实现。

为了防止并发导致的竞争问题，程序共设计了两种粒度的锁：一种是基于每个 key 加锁，用于多个 reader 和 writer 间的互斥性，并且保证了在读取和写入过程中的并发度，具体由 concurrency 库中提供的 Lock 和 UnLock API 实现；另一种是对 ETCD 中的所有数据上锁，用于 reseter 在清空所有数据的过程中，阻止 reader 和 writer 读取或写入错的数据，由于 concurrency  中不能提供仅查看锁而不真正上锁这一功能，我们采用对某一全局键值对进行设置的方法实现锁的功能，当 value 为 0 时证明 reseter 没有运行，可以进行读取操作，value 为 1 时 reseter 正在运行，禁止读取操作。

除此之外，为了防止 reseter 意外退出，还需要在程序中加入优雅退出机制，目的是接收各种程序退出的信号，并在收到信号后将键值对中的 value 设置为 0，防止程序在重启后进入死锁状态。

## 细节说明

- **ETCD 中键值对的设置**

  **键值对**：key 的前缀为 /globalmap/，value为原值，如键值对 zecrey/is amazing，存入 ETCD 后 key 为 /globalmap/zecrey，value 为 is amazing。

  **Lock**：每个键值对都会建立对应一个 lock，在ETCD中，Lock 的结构也是键值对，前缀为 /lock/，如键值对 zecrey/is amazing，其对应的 Lock 为 /lock/zecrey。

- **Reseter意外退出的信号处理**

  reseter 中用于锁住全局的键值对中的 key 就是 /lock/，value 是 0或1。

  由于在进入 reseter 时会将 value 设置为 1，表示全局上锁，此时若 reseter 崩溃则会导致 value 一直为 1，形成死锁，可以通过 `etcdctl get /lock/` 进行查看，`etcdctl put /lock/ 0` 进行设置。所以需要在程序崩溃退出之前启动优雅退出机制，根据信号调用信号处理函数，将 value 重新设置回 0。目前监听了SIGHUP, SIGINT, SIGTERM, SIGQUIT 几种信号。

- **正确性测试**

  测试中曾经会出现锁无效、死锁、租期队列过多、ETCD宕机，当前版本中这些问题都已经解决。测试环境应在启动多个的程序，模拟多个 reader，writer 和 reseter 在随机运行的情况，验证在长时间高频率调用中，是否保证了数据的正确、高效和稳定性。

- **大数据量测试**
