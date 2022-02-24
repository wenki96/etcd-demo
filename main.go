package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// func hash(s string) uint32 {
//  h := fnv.New32a()
//  h.Write([]byte(s))
//  return h.Sum32()
// }

var (
	key           = "/distributed/etcd/"
	prefixKey     = "/distributed/"
	keyLock       = "/distributed-lock/etcd/"
	prefixKeyLock = "/distributed-lock/"
	value         = "helloworld"
)

// var lReset = &concurrency.Mutex{}

func main() {

	// read key /distributed/etcd
	for i := 1; i < 10; i++ {
		go reader(key, keyLock, i)
	}

	// write key /distributed/etcd
	go writer(key, keyLock, value)

	// write prefix key /distributed/
	go reseter(key, prefixKeyLock, prefixKey)

	select {}
}

func resetKeyUsed(cli *clientv3.Client) bool {
	var getResp *clientv3.GetResponse
	// 实例化一个用于操作ETCD的KV
	kv := clientv3.NewKV(cli)

	getResp, err := kv.Get(context.TODO(), prefixKeyLock)
	if err != nil {
		fmt.Println(err)
		return true
	}

	// 输出本次的Revision
	if getResp.Kvs != nil {
		// fmt.Println(getResp.Kvs[0].Value)
		return string(getResp.Kvs[0].Value) == "1"
	}

	return false
}

func reader(key, keyLock string, readerNum int) {
	// Create a etcd client
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	for {
		// concurrency read

		// whether reseter hold the lock
		if resetKeyUsed(cli) {
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
		if resetKeyUsed(cli) {
			// fmt.Printf("Reseter holds the lock [Reader]: Reader  %d\n", readerNum)
			if err := l.Unlock(ctx); err != nil {
				log.Fatal(err)
			}
			continue
		}

		fmt.Printf("[Reader] Reader  %d\n", readerNum)
		fmt.Println("acquired lock for read ", keyLock)

		// time.Sleep(1 * time.Second)

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

func writer(key, keyLock, value string) {
	// Create a etcd client
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	i := 1
	for {
		// concurrency write

		// whether reseter hold the lock
		if resetKeyUsed(cli) {
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
		if resetKeyUsed(cli) {
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

		// time.Sleep(2 * time.Second)

		if err := l.Unlock(ctx); err != nil {
			log.Fatal(err)
		}

		fmt.Println("released lock for write ", keyLock)
		fmt.Println()

		i++
	}
}

func setResetKey(cli *clientv3.Client, key string) {
	kv := clientv3.NewKV(cli)
	if _, err := kv.Put(context.TODO(), prefixKeyLock, key, clientv3.WithPrevKV()); err != nil {
		fmt.Println(err)
		return
	}
}

func ExitFunc() {
	os.Exit(0)
}

func reseter(key, prefixKeyLock, prefixKey string) {
	// Create a etcd client
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	c := make(chan os.Signal)
	// 监听信号
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
				fmt.Println("退出:", s)
				setResetKey(cli, "0")
				ExitFunc()
			default:
				fmt.Println("其他信号:", s)
			}
		}
	}()

	// concurrency reset
	for {
		time.Sleep(5 * time.Second)
		// acquire lock (or wait to have it)

		setResetKey(cli, "1")

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
		time.Sleep(10 * time.Second)

		setResetKey(cli, "0")

		fmt.Println("released lock for reset ", prefixKeyLock)
		fmt.Println()

	}
}
