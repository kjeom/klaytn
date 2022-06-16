// Modifications Copyright 2018 The klaytn Authors
// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from ethdb/database_test.go (2018/06/04).
// Modified and improved for the klaytn development.

package database

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/klaytn/klaytn/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func newTestLDB() (Database, func()) {
	dirName, err := ioutil.TempDir(os.TempDir(), "klay_leveldb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := NewLevelDBWithOption(dirName, GetDefaultLevelDBOption())
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirName)
	}
}

func newTestBadgerDB() (Database, func()) {
	dirName, err := ioutil.TempDir(os.TempDir(), "klay_badgerdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := NewBadgerDB(dirName)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirName)
	}
}

func newTestMemDB() (Database, func()) {
	return NewMemDB(), func() {}
}

func newTestDynamoS3DB() (Database, func()) {
	// to start test with DynamoDB singletons
	oldDynamoDBClient := dynamoDBClient
	dynamoDBClient = nil

	oldDynamoOnceWorker := dynamoOnceWorker
	dynamoOnceWorker = &sync.Once{}

	oldDynamoWriteCh := dynamoWriteCh
	dynamoWriteCh = nil

	db, err := newDynamoDB(GetTestDynamoConfig())
	if err != nil {
		panic("failed to create test DynamoS3 database: " + err.Error())
	}
	return db, func() {
		db.Close()
		db.deleteTable()
		db.fdb.deleteBucket()

		// to finish test with DynamoDB singletons
		dynamoDBClient = oldDynamoDBClient
		dynamoOnceWorker = oldDynamoOnceWorker
		dynamoWriteCh = oldDynamoWriteCh
	}
}

type commonDatabaseTestSuite struct {
	suite.Suite
	database Database
}

func TestDatabaseTestSuite(t *testing.T) {
	// If you want to include dynamo test, use below line
	// var testDatabases = []func() (Database, func()){newTestLDB, newTestBadgerDB, newTestMemDB, newTestDynamoS3DB}

	// TODO-Klaytn-Database Need to add DynamoDB to the below list.
	testDatabases := []func() (Database, func()){newTestLDB, newTestBadgerDB, newTestMemDB}
	for _, newFn := range testDatabases {
		db, remove := newFn()
		suite.Run(t, &commonDatabaseTestSuite{database: db})
		remove()
	}
}

// TestNilValue checks if all database write/read nil value in the same way.
func (ts *commonDatabaseTestSuite) TestNilValue() {
	db, t := ts.database, ts.T()

	// non-batch
	{
		// write nil value
		key := common.MakeRandomBytes(32)
		assert.Nil(t, db.Put(key, nil))

		// get nil value
		ret, err := db.Get(key)
		assert.Equal(t, []byte{}, ret)
		assert.Nil(t, err)

		// check existence
		exist, err := db.Has(key)
		assert.Equal(t, true, exist)
		assert.Nil(t, err)

		val, err := db.Get(randStrBytes(100))
		assert.Nil(t, val)
		assert.Error(t, err)
		assert.Equal(t, dataNotFoundErr, err)
	}

	// batch
	{
		batch := db.NewBatch()

		// write nil value
		key := common.MakeRandomBytes(32)
		assert.Nil(t, batch.Put(key, nil))
		assert.NoError(t, batch.Write())

		// get nil value
		ret, err := db.Get(key)
		assert.Equal(t, []byte{}, ret)
		assert.Nil(t, err)

		// check existence
		exist, err := db.Has(key)
		assert.Equal(t, true, exist)
		assert.Nil(t, err)

		val, err := db.Get(randStrBytes(100))
		assert.Nil(t, val)
		assert.Error(t, err)
		assert.Equal(t, dataNotFoundErr, err)
	}
}

// TestNotFoundErr checks if an empty database returns DataNotFoundErr for the given random key.
func (ts *commonDatabaseTestSuite) TestNotFoundErr() {
	db, t := ts.database, ts.T()

	val, err := db.Get(randStrBytes(100))
	assert.Nil(t, val)
	assert.Error(t, err)
	assert.Equal(t, dataNotFoundErr, err)
}

// TestPutGet tests the basic put and get operations.
func (ts *commonDatabaseTestSuite) TestPutGet() {
	db, t := ts.database, ts.T()

	// Since badgerDB can't store empty key, testValues is modified. Below line is the original testValues.
	// var testValues = []string{"", "a", "1251", "\x00123\x00"}
	testValues := []string{"a", "1251", "\x00123\x00"}

	// put
	for _, v := range testValues {
		err := db.Put([]byte(v), []byte(v))
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	// get
	for _, v := range testValues {
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte(v)) {
			t.Fatalf("get returned wrong result, got %q expected %q", string(data), v)
		}
	}

	// override with "?"
	for _, v := range testValues {
		err := db.Put([]byte(v), []byte("?"))
		if err != nil {
			t.Fatalf("put override failed: %v", err)
		}
	}

	// get "?" by key
	for _, v := range testValues {
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	// override returned value
	for _, v := range testValues {
		orig, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		orig[0] = byte(0xff)
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	// delete
	for _, v := range testValues {
		err := db.Delete([]byte(v))
		if err != nil {
			t.Fatalf("delete %q failed: %v", v, err)
		}
	}

	// try to get deleted values
	for _, v := range testValues {
		_, err := db.Get([]byte(v))
		if err == nil {
			t.Fatalf("got deleted value %q", v)
		}
	}
}

func TestShardDB(t *testing.T) {
	key := common.Hex2Bytes("0x91d6f7d2537d8a0bd7d487dcc59151ebc00da306")

	hashstring := strings.TrimPrefix("0x93d6f3d2537d8a0bd7d485dcc59151ebc00da306", "0x")
	if len(hashstring) > 15 {
		hashstring = hashstring[:15]
	}
	seed, _ := strconv.ParseInt(hashstring, 16, 64)

	shard := seed % int64(12)

	idx := common.BytesToHash(key).Big().Mod(common.BytesToHash(key).Big(), big.NewInt(4))

	fmt.Printf("idx %d   %d   %d\n", idx, shard, seed)
}

// TestParallelPutGet tests the parallel put and get operations.
func (ts *commonDatabaseTestSuite) TestParallelPutGet() {
	db := ts.database
	const n = 8
	var pending sync.WaitGroup

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Put([]byte(key), []byte("v"+key))
			if err != nil {
				panic("put failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			data, err := db.Get([]byte(key))
			if err != nil {
				panic("get failed: " + err.Error())
			}
			if !bytes.Equal(data, []byte("v"+key)) {
				panic(fmt.Sprintf("get failed, got %q expected %q", []byte(data), []byte("v"+key)))
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Delete([]byte(key))
			if err != nil {
				panic("delete failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			_, err := db.Get([]byte(key))
			if err == nil {
				panic("got deleted value")
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()
}

// TestDBEntryLengthCheck checks if dbDirs and dbConfigRatio are
// specified for every DBEntryType.
func TestDBEntryLengthCheck(t *testing.T) {
	dbRatioSum := 0
	for i := 0; i < int(databaseEntryTypeSize); i++ {
		if dbBaseDirs[i] == "" {
			t.Fatalf("Database directory should be specified! index: %v", i)
		}

		if dbConfigRatio[i] == 0 {
			t.Fatalf("Database configuration ratio should be specified! index: %v", i)
		}

		dbRatioSum += dbConfigRatio[i]
	}

	if dbRatioSum != 100 {
		t.Fatalf("Sum of database configuration ratio should be 100! actual: %v", dbRatioSum)
	}
}
