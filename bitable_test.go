// Copyright 2019 The Bitalostored Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package bitalostable

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
)

const (
	testDir = "./data"

	maxLogFileSize       = 128 << 20
	memTableSize         = 1 << 20
	maxWriteBufferNumber = 3

	valrandStr = "1qaz2wsx3edc4rfv5tgb6yhn7ujm8ik9ol0p"
)

type BitableDB struct {
	db   *DB
	ro   *IterOptions
	wo   *WriteOptions
	opts *Options
}

func testRandBytes(n int, randstr string) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = randstr[rand.Int63()%int64(len(randstr))]
	}
	return b
}

func openBitable(dir string, walDir string) (*BitableDB, error) {
	opts := &Options{
		MaxManifestFileSize:         maxLogFileSize,
		MemTableSize:                memTableSize,
		MemTableStopWritesThreshold: maxWriteBufferNumber,
		Verbose:                     true,
	}
	return openBitableByOpts(dir, walDir, opts)
}

func openBitableByOpts(dir string, walDir string, opts *Options) (*BitableDB, error) {
	cache := NewCache(0)
	opts.Cache = cache
	if len(walDir) > 0 {
		_, err := os.Stat(walDir)
		if nil != err && !os.IsExist(err) {
			err = os.MkdirAll(walDir, 0775)
			if nil != err {
				return nil, err
			}
			opts.WALDir = walDir
		}
	}
	_, err := os.Stat(dir)
	if nil != err && !os.IsExist(err) {
		err = os.MkdirAll(dir, 0775)
		if nil != err {
			return nil, err
		}
		opts.WALDir = walDir
	}
	pdb, err := Open(dir, opts)
	if err != nil {
		return nil, err
	}
	cache.Unref()
	return &BitableDB{
		db:   pdb,
		ro:   &IterOptions{},
		wo:   NoSync,
		opts: opts,
	}, nil
}

func TestBitable_MemIterator(t *testing.T) {
	defer os.RemoveAll(testDir)
	bitalostableDB, err := openBitable(testDir, "")
	if err != nil {
		panic(err)
	}
	defer func() {
		require.NoError(t, bitalostableDB.db.Close())
	}()

	for i := 0; i < 100; i++ {
		newKey := []byte(fmt.Sprintf("quota:host:province:succ:total_%d", i))
		for j := 0; j < 100; j++ {
			err = bitalostableDB.db.Set(newKey, []byte(fmt.Sprintf("%d", j)), bitalostableDB.wo)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	it := bitalostableDB.db.NewIter(nil)
	defer it.Close()
	for it.First(); it.Valid(); it.Next() {
		//fmt.Println("iter", string(it.Key()), string(it.Value()))
	}
	stats := it.Stats()
	fmt.Printf("stats: %s\n", stats.String())
}

func TestBitable_MemGet(t *testing.T) {
	defer os.RemoveAll(testDir)
	bitalostableDB, err := openBitable(testDir, "")
	if err != nil {
		panic(err)
	}
	defer func() {
		require.NoError(t, bitalostableDB.db.Close())
	}()

	newKey := []byte(fmt.Sprintf("quota:host:province:succ:total_1"))
	for j := 0; j < 100; j++ {
		err = bitalostableDB.db.Set(newKey, []byte(fmt.Sprintf("%d", j)), bitalostableDB.wo)
		if err != nil {
			t.Fatal(err)
		}
	}

	require.NoError(t, bitalostableDB.db.Flush())

	val, i, err := bitalostableDB.db.Get(newKey)
	require.Equal(t, []byte(fmt.Sprintf("%d", 99)), val)
	defer func() {
		if i != nil {
			require.NoError(t, i.Close())
		}
	}()

	it := i.(*Iterator)
	stats := it.Stats()
	fmt.Printf("stats: %s\n", stats.String())
}

func TestBitable_NewFlushBatch_DisableCompact(t *testing.T) {
	for _, isCompact := range []bool{false} {
		fmt.Println("start-------------", isCompact)
		func() {
			defer os.RemoveAll(testDir)
			os.RemoveAll(testDir)

			opts := &Options{
				MaxManifestFileSize:         maxLogFileSize,
				MemTableSize:                memTableSize,
				MemTableStopWritesThreshold: 8,
				L0CompactionFileThreshold:   2,
				L0CompactionThreshold:       2,
				L0StopWritesThreshold:       128,
				Verbose:                     true,
			}
			bitalostableDB, err := openBitableByOpts(testDir, "", opts)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, bitalostableDB.db.Close())
			}()

			inum := 1
			jnum := 10000
			value := testRandBytes(2048, valrandStr)

			writeData := func() {
				batchSize := 5 << 20
				batch := bitalostableDB.db.NewFlushBatch(batchSize)
				for i := 0; i < inum; i++ {
					for j := 0; j < jnum; j++ {
						newKey := []byte(fmt.Sprintf("key_%d_%d", i, j))
						err = batch.Set(newKey, value, bitalostableDB.wo)
						if err != nil {
							t.Fatal(err)
						}
					}
					fmt.Println("batch.Commit")
					err = batch.Commit(bitalostableDB.wo)
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, batch.Close())
					batch = bitalostableDB.db.NewFlushBatch(batchSize)
				}
				require.NoError(t, batch.Close())
			}

			readData := func() {
				for i := 0; i < inum; i++ {
					for j := 0; j < jnum; j++ {
						newKey := []byte(fmt.Sprintf("key_%d_%d", i, j))
						val, closer, err := bitalostableDB.db.Get(newKey)
						if err != nil {
							t.Error("get err", err, string(newKey))
						} else if !bytes.Equal(value, val) {
							t.Error("get val err", string(newKey))
						}
						if closer != nil {
							require.NoError(t, closer.Close())
						}
					}
				}
			}

			bitalostableDB.db.SetOptsDisableAutomaticCompactions(true)
			writeData()
			readData()
			time.Sleep(time.Second)
			fmt.Println(bitalostableDB.db.Metrics().String())

			bitalostableDB.db.SetOptsDisableAutomaticCompactions(false)

			if isCompact {
				require.NoError(t, bitalostableDB.db.Compact(nil, []byte("\xff"), false))
			} else {
				writeData()
				time.Sleep(time.Second)
				fmt.Println(bitalostableDB.db.Metrics().String())
			}

			readData()
		}()
	}
}

func TestBitable_NewBatch(t *testing.T) {
	defer os.RemoveAll(testDir)
	os.RemoveAll(testDir)
	bitalostableDB, err := openBitable(testDir, "")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bitalostableDB.db.Close())
	}()

	inum := 1
	jnum := 1000

	batch := bitalostableDB.db.NewBatch()
	value := testRandBytes(2048, valrandStr)
	for i := 0; i < inum; i++ {
		for j := 0; j < jnum; j++ {
			newKey := []byte(fmt.Sprintf("key_%d_%d", i, j))
			err = batch.Set(newKey, value, bitalostableDB.wo)
			if err != nil {
				t.Fatal(err)
			}
		}
		err = batch.Commit(bitalostableDB.wo)
		if err != nil {
			t.Fatal(err)
		}
		batch.Close()
		batch = bitalostableDB.db.NewBatch()
	}
	batch.Close()

	fmt.Println(bitalostableDB.db.Metrics().String())

	for i := 0; i < inum; i++ {
		for j := 0; j < jnum; j++ {
			newKey := []byte(fmt.Sprintf("key_%d_%d", i, j))
			val, closer, err := bitalostableDB.db.Get(newKey)
			if err != nil {
				t.Error("get err", err, string(newKey))
			} else if len(val) <= 0 {
				t.Error("get val len err")
			} else if !bytes.Equal(value, val) {
				t.Error("get val err", string(newKey))
			}
			if closer != nil {
				require.NoError(t, closer.Close())
			}
		}
	}
}

func TestBitable_BatchSetMulti(t *testing.T) {
	defer os.RemoveAll(testDir)
	os.RemoveAll(testDir)
	bitalostableDB, err := openBitable(testDir, "")
	require.NoError(t, err)

	kvList := make(map[string][]byte, 0)

	for i := 0; i < 100; i++ {
		b := bitalostableDB.db.NewBatch()
		key := []byte(fmt.Sprintf("key_%d", i))
		if i%2 == 0 {
			val := testRandBytes(100, valrandStr)
			_ = b.Set(key, val, bitalostableDB.wo)
			kvList[string(key)] = val
		} else {
			val1 := testRandBytes(100, valrandStr)
			val2 := testRandBytes(100, valrandStr)
			_ = b.SetMultiValue(key, val1, val2)
			var val []byte
			val = append(val, val1...)
			val = append(val, val2...)
			kvList[string(key)] = val
		}
		require.NoError(t, b.Commit(bitalostableDB.wo))
		require.NoError(t, b.Close())
	}

	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		v, vcloser, err := bitalostableDB.db.Get(key)
		require.NoError(t, err)
		require.Equal(t, kvList[string(key)], v)
		require.NoError(t, vcloser.Close())
	}

	require.NoError(t, bitalostableDB.db.Close())
}

func TestBitable_Compact_CheckExpire(t *testing.T) {
	defer os.RemoveAll(testDir)
	os.RemoveAll(testDir)

	opts := &Options{
		MaxManifestFileSize:         maxLogFileSize,
		MemTableSize:                memTableSize,
		MemTableStopWritesThreshold: maxWriteBufferNumber,
		L0CompactionFileThreshold:   8,
		L0CompactionThreshold:       8,
		L0StopWritesThreshold:       16,
		Verbose:                     true,
		KvCheckExpireFunc: func(k, v []byte) bool {
			if v != nil && uint8(v[0]) == 1 {
				timestamp := binary.BigEndian.Uint64(v[1:9])
				if timestamp == 0 {
					return false
				}
				now := uint64(time.Now().UnixMilli())
				return timestamp <= now
			}
			return false
		},
	}
	bitalostableDB, err := openBitableByOpts(testDir, "", opts)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bitalostableDB.db.Close())
	}()

	now := uint64(time.Now().UnixMilli())
	fmt.Println("now time", now)
	makeValue := func(i int, valBytes []byte) []byte {
		var val []byte
		var ttl uint64
		if i%5 == 0 {
			ttl = now + 2000
		} else {
			ttl = now + 100000
		}
		val = make([]byte, len(valBytes)+9)
		val[0] = 1
		binary.BigEndian.PutUint64(val[1:9], ttl)
		copy(val[9:], valBytes)
		return val
	}

	num := 10000
	value := testRandBytes(1024, valrandStr)

	writeData := func() {
		for j := 0; j < num; j++ {
			newKey := []byte(fmt.Sprintf("key_%d", j))
			err = bitalostableDB.db.Set(newKey, makeValue(j, value), bitalostableDB.wo)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	readData := func(isDel bool) {
		for i := 0; i < num; i++ {
			newKey := []byte(fmt.Sprintf("key_%d", i))
			val, closer, err := bitalostableDB.db.Get(newKey)
			if isDel && i%5 == 0 {
				if err != ErrNotFound {
					t.Fatal("find expire key", string(newKey))
				}
			} else {
				if err != nil {
					t.Fatal("find not expire key err", string(newKey), err)
				} else if !bytes.Equal(makeValue(i, value), val) {
					t.Fatal("find not expire key val err", string(newKey))
				}
			}
			if closer != nil {
				require.NoError(t, closer.Close())
			}
		}
	}

	writeData()
	readData(false)
	fmt.Println("---------wr 1")
	time.Sleep(2 * time.Second)
	require.NoError(t, bitalostableDB.db.Compact(nil, []byte("\xff"), false))
	readData(true)
}
