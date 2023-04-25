// Copyright 2018 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package concurrency_test

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/ambition9186/etcd/clientv3"
	"github.com/ambition9186/etcd/clientv3/concurrency"
)

func TestResumeElection(t *testing.T) {
	const prefix = "/resume-election/"

	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	s, err := concurrency.NewSession(cli)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	e := concurrency.NewElection(s, prefix)

	// Entire test should never take more than 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Become leader
	if err := e.Campaign(ctx, "candidate1"); err != nil {
		t.Fatalf("Campaign() returned non nil err: %s", err)
	}

	// Get the leadership details of the current election
	leader, err := e.Leader(ctx)
	if err != nil {
		t.Fatalf("Leader() returned non nil err: %s", err)
	}

	// Recreate the election
	e = concurrency.ResumeElection(s, prefix,
		string(leader.Kvs[0].Key), leader.Kvs[0].CreateRevision)

	respChan := make(chan *clientv3.GetResponse)
	go func() {
		o := e.Observe(ctx)
		respChan <- nil
		for {
			select {
			case resp, ok := <-o:
				if !ok {
					t.Fatal("Observe() channel closed prematurely")
				}
				// Ignore any observations that candidate1 was elected
				if string(resp.Kvs[0].Value) == "candidate1" {
					continue
				}
				respChan <- &resp
				return
			}
		}
	}()

	// Wait until observe goroutine is running
	<-respChan

	// Put some random data to generate a change event, this put should be
	// ignored by Observe() because it is not under the election prefix.
	_, err = cli.Put(ctx, "foo", "bar")
	if err != nil {
		t.Fatalf("Put('foo') returned non nil err: %s", err)
	}

	// Resign as leader
	if err := e.Resign(ctx); err != nil {
		t.Fatalf("Resign() returned non nil err: %s", err)
	}

	// Elect a different candidate
	if err := e.Campaign(ctx, "candidate2"); err != nil {
		t.Fatalf("Campaign() returned non nil err: %s", err)
	}

	// Wait for observed leader change
	resp := <-respChan

	kv := resp.Kvs[0]
	if !strings.HasPrefix(string(kv.Key), prefix) {
		t.Errorf("expected observed election to have prefix '%s' got '%s'", prefix, string(kv.Key))
	}
	if string(kv.Value) != "candidate2" {
		t.Errorf("expected new leader to be 'candidate1' got '%s'", string(kv.Value))
	}
}
