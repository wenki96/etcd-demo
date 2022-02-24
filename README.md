# 流程说明

整体说明：demo中实现了基于ETCD的分布式存储和分布式锁功能，满足两种不同粒度（单key/全局）下的数据读取和写入功能，并通过分布式锁保证其互斥性。

程序共包含三种函数：reader，writer和 reseter。其中 reader 负责从 ETCD 中根据 key 读取对应的 value，由get 实现；writer 负责向 ETCD 中加入或更新某个 key/value 键值对，由 put 实现；reseter 负责重置 ETCD 中的所有键值对，由 delete 前缀值实现。

为了防止并发导致的竞争问题，程序共设计了两种粒度的锁：一种是基于每个 key 加锁，用于多个 reader 和 writer 间的互斥性，并且保证了在读取和写入过程中的并发度，具体由 concurrency 库中提供的 Lock 和 UnLock API 实现；另一种是对 ETCD 中的所有数据上锁，用于 reseter 在清空所有数据的过程中，阻止 reader 和 writer 读取或写入错的数据，由于 concurrency  中不能提供仅查看锁而不真正上锁这一功能，我们采用对某一全局键值对进行设置的方法实现锁的功能，当 value 为 0 时证明 reseter 没有运行，可以进行读取操作，value 为 1 时 reseter 正在运行，禁止读取操作。

除此之外，为了防止 reseter 意外退出，还需要在程序中加入优雅退出机制，目的是接收各种程序退出的信号，并在收到信号后将键值对中的 value 设置为 0，防止程序在重启后进入死锁状态。
