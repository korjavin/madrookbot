package main

import (
	"encoding/binary"
	// "encoding/json"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
)

var db *bolt.DB

// type Prefs struct {
// 	voice string `json:"voice"`
// }

func init() {
	var err error
	db, err = bolt.Open("my.db", 0o666, &bolt.Options{ReadOnly: false})
	if err != nil {
		log.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func loadprefs() error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))

		err := b.ForEach(func(k, v []byte) error {
			// var p Prefs
			// err := json.Unmarshal(v, &p)
			// if err != nil {
			// 	fmt.Println("error:", err)
			// }
			log.Printf("key=%d, value=%v\n", btoi(k), string(v))
			prefs[btoi(k)] = string(v)
			return nil
		})
		return err
	})
}

func saveprefs(user int, p string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		// buf, err := json.Marshal(p)
		// if err != nil {
		// 	return err
		// }

		log.Printf("key=%d, value=%#v  \n", user, p)
		return b.Put(itob(user), []byte(p))
	})
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func btoi(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}
