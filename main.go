package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

// func hash(s string) uint32 {
//  h := fnv.New32a()
//  h.Write([]byte(s))
//  return h.Sum32()
// }

func main() {
	// Create a etcd client
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	key := "/distributed/etcd/"
	prefixKey := "/distributed/"
	keyLock := "/distributed-lock/etcd/"
	prefixKeyLock := "/distributed-lock/"
	value := "helloworld"

	// create a sessions to aqcuire a lock for reset
	sReset, err := concurrency.NewSession(cli, concurrency.WithTTL(10))
	if err != nil {
		log.Fatal(err)
	}
	defer sReset.Close()
	ctxReset := context.Background()
	lReset := concurrency.NewMutex(sReset, prefixKeyLock)

	// read key /distributed/etcd
	for i := 1; i < 10; i++ {
		go reader(cli, key, keyLock, i, lReset)
	}

	// write key /distributed/etcd
	go writer(cli, key, keyLock, value, lReset)

	// write prefix key /distributed/
	go reseter(cli, key, prefixKeyLock, prefixKey, ctxReset, lReset)

	select {}
}

func reader(cli *clientv3.Client, key, keyLock string, readerNum int, lReset *concurrency.Mutex) {
	for {
		// concurrency read

		// whether reseter hold the lock
		if len(lReset.IsOwner().Key) > 1 {
			// fmt.Printf("Reseter holds the lock [Reader]: Reader  %d\n", readerNum)
			continue
		}

		// create a sessions to aqcuire a lock
		s, err := concurrency.NewSession(cli, concurrency.WithTTL(10))
		if err != nil {
			log.Fatal(err)
		}
		defer s.Close()

		ctx := context.Background()

		l := concurrency.NewMutex(s, keyLock)

		// acquire lock (or wait to have it)
		if err := l.Lock(ctx); err != nil {
			log.Fatal(err)
		}

		// unlock
		if len(lReset.IsOwner().Key) > 1 {
			// fmt.Printf("Reseter holds the lock [Reader]: Reader  %d\n", readerNum)
			if err := l.Unlock(ctx); err != nil {
				log.Fatal(err)
			}
			continue
		}

		fmt.Printf("[Reader] Reader  %d\n", readerNum)
		fmt.Println("acquired lock for read ", keyLock)

		time.Sleep(1 * time.Second)

		var getResp *clientv3.GetResponse
		// 实例化一个用于操作ETCD的KV
		kv := clientv3.NewKV(cli)

		if getResp, err = kv.Get(context.TODO(), key); err != nil {
			fmt.Println(err)
			return
		}

		// 输出本次的Revision
		if getResp.Kvs != nil {
			fmt.Printf("Key is %s Value is %s \n", getResp.Kvs[0].Key, getResp.Kvs[0].Value)
		}

		if err := l.Unlock(ctx); err != nil {
			log.Fatal(err)
		}

		fmt.Println("released lock for read ", keyLock)
		fmt.Println()
	}
}

func writer(cli *clientv3.Client, key, keyLock, value string, lReset *concurrency.Mutex) {
	i := 1
	for {
		// concurrency write

		// whether reseter hold the lock
		if len(lReset.IsOwner().Key) > 1 {
			// fmt.Printf("Reseter holds the lock [Writer]\n")
			continue
		}

		// create a sessions to aqcuire a lock
		s, err := concurrency.NewSession(cli, concurrency.WithTTL(10))
		if err != nil {
			log.Fatal(err)
		}
		defer s.Close()

		ctx := context.Background()

		l := concurrency.NewMutex(s, keyLock)

		// acquire lock (or wait to have it)
		if err := l.Lock(ctx); err != nil {
			log.Fatal(err)
		}

		// unlock
		if len(lReset.IsOwner().Key) > 1 {
			// fmt.Printf("Reseter holds the lock [Reader]: Reader  %d\n", readerNum)
			if err := l.Unlock(ctx); err != nil {
				log.Fatal(err)
			}
			continue
		}

		fmt.Printf("[Writer]: \n")
		fmt.Println("acquired lock for write ", keyLock)

		var putResp *clientv3.PutResponse
		// 实例化一个用于操作ETCD的KV
		kv := clientv3.NewKV(cli)

		if putResp, err = kv.Put(context.TODO(), key, fmt.Sprintf("%s:%d", value, i), clientv3.WithPrevKV()); err != nil {
			fmt.Println(err)
			return
		}
		// fmt.Println(putResp.Header.Revision)
		if putResp.PrevKv != nil {
			fmt.Printf("preValue: %s CreateRevision : %d  ModRevision: %d  Version: %d \n",
				putResp.PrevKv.Value, putResp.PrevKv.CreateRevision, putResp.PrevKv.ModRevision, putResp.PrevKv.Version)
		}
		fmt.Println("curValue: ", fmt.Sprintf("%s%d", value, i))

		time.Sleep(2 * time.Second)

		if err := l.Unlock(ctx); err != nil {
			log.Fatal(err)
		}

		fmt.Println("released lock for write ", keyLock)
		fmt.Println()

		i++
	}
}

func reseter(cli *clientv3.Client, key, prefixKeyLock, prefixKey string, ctx context.Context, l *concurrency.Mutex) {
	// concurrency reset
	for {
		time.Sleep(15 * time.Second)
		// acquire lock (or wait to have it)
		if err := l.Lock(ctx); err != nil {
			log.Fatal(err)
		}

		fmt.Println("[Reseter]")
		fmt.Println("acquired lock for reset ", prefixKeyLock)

		kv := clientv3.NewKV(cli)
		res, err := kv.Delete(context.TODO(), prefixKey, clientv3.WithPrevKV(), clientv3.WithPrefix())
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("delete %d keys\n", res.Deleted)
			for _, preKv := range res.PrevKvs {
				fmt.Printf("del key: %s, value: %s\n", preKv.Key, preKv.Value)
			}
		}
		time.Sleep(20 * time.Second)

		if err := l.Unlock(ctx); err != nil {
			log.Fatal(err)
		}

		fmt.Println("released lock for reset ", prefixKeyLock)
		fmt.Println()

	}
}
