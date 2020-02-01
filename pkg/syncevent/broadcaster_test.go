// Copyright 2020 The gVisor Authors.
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

package syncevent

import (
	"fmt"
	"math/rand"
	"testing"

	"gvisor.dev/gvisor/pkg/sync"
	"gvisor.dev/gvisor/pkg/waiter"
)

func TestBroadcasterFilter(t *testing.T) {
	const numReceivers = 2 * MaxEvents

	var br Broadcaster
	ws := make([]Waiter, numReceivers)
	for i := range ws {
		ws[i].Init()
		br.SubscribeEvents(ws[i].Receiver(), 1<<(i%MaxEvents))
	}
	for ev := 0; ev < MaxEvents; ev++ {
		br.Broadcast(1 << ev)
		for i := range ws {
			want := NoEvents
			if i%MaxEvents == ev {
				want = 1 << ev
			}
			if got := ws[i].Pending(); got != want {
				t.Errorf("after Broadcast of event %d: waiter %d has pending event set %#x, wanted %#x", ev, i, got, want)
			}
			ws[i].Ack(AllEvents)
		}
	}
}

// TestBroadcasterManySubscriptions tests that subscriptions are not lost by
// table expansion/compaction.
func TestBroadcasterManySubscriptions(t *testing.T) {
	const numReceivers = 5000 // arbitrary

	var br Broadcaster
	ws := make([]Waiter, numReceivers)
	for i := range ws {
		ws[i].Init()
	}

	ids := make([]SubscriptionID, numReceivers)
	for i := 0; i < numReceivers; i++ {
		// Subscribe receiver i.
		ids[i] = br.SubscribeEvents(ws[i].Receiver(), 1)
		// Check that receivers [0, i] are subscribed.
		br.Broadcast(1)
		for j := 0; j <= i; j++ {
			if ws[j].Pending() != 1 {
				t.Errorf("receiver %d did not receive an event after subscription of receiver %d", j, i)
			}
			ws[j].Ack(1)
		}
	}

	for i := 0; i < numReceivers; i++ {
		// Unsubscribe receiver i.
		br.UnsubscribeEvents(ids[i])
		// Check that receivers [0, i] are not subscribed, and that receivers
		// (i, numReceivers) are still subscribed.
		br.Broadcast(1)
		for j := 0; j <= i; j++ {
			if ws[j].Pending() != 0 {
				t.Errorf("receiver %d received an event after unsubscription of receiver %d", j, i)
			}
		}
		for j := i + 1; j < numReceivers; j++ {
			if ws[j].Pending() != 1 {
				t.Errorf("receiver %d did not receive an event after unsubscription of receiver %d", j, i)
			}
			ws[j].Ack(1)
		}
	}
}

var (
	receiverCountsNonZero       = []int{1, 4, 16, 64}
	receiverCountsIncludingZero = append([]int{0}, receiverCountsNonZero...)
)

// BenchmarkBroadcasterX, BenchmarkMapX, and BenchmarkQueueX benchmark usage
// pattern X (described in terms of Broadcaster) with Broadcaster, a
// Mutex-protected map[*Receiver]Set, and waiter.Queue respectively.

// Benchmark*SubscribeUnsubscribe measures the cost of a Subscribe/Unsubscribe
// cycle.

func BenchmarkBroadcasterSubscribeUnsubscribe(b *testing.B) {
	var br Broadcaster
	var w Waiter
	w.Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := br.SubscribeEvents(w.Receiver(), 1)
		br.UnsubscribeEvents(id)
	}
}

func BenchmarkMapSubscribeUnsubscribe(b *testing.B) {
	var mu sync.Mutex
	m := make(map[*Receiver]Set)
	var w Waiter
	w.Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m[w.Receiver()] = Set(1)
		mu.Unlock()
		mu.Lock()
		delete(m, w.Receiver())
		mu.Unlock()
	}
}

func BenchmarkQueueSubscribeUnsubscribe(b *testing.B) {
	var q waiter.Queue
	e, _ := waiter.NewChannelEntry(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.EventRegister(&e, 1)
		q.EventUnregister(&e)
	}
}

// Benchmark*SubscribeUnsubscribeBatch is similar to
// Benchmark*SubscribeUnsubscribe, but subscribes and unsubscribes a large
// number of Receivers at a time in order to measure the amortized overhead of
// table expansion/compaction. (Since waiter.Queue is implemented using a
// linked list, BenchmarkQueueSubscribeUnsubscribe and
// BenchmarkQueueSubscribeUnsubscribeBatch should produce nearly the same
// result..)

const numBatchReceivers = 1000

func BenchmarkBroadcasterSubscribeUnsubscribeBatch(b *testing.B) {
	var br Broadcaster
	ws := make([]Waiter, numBatchReceivers)
	for i := range ws {
		ws[i].Init()
	}
	ids := make([]SubscriptionID, numBatchReceivers)

	// Generate a random order for unsubscriptions.
	unsubIndex := make([]int, numBatchReceivers)
	for i := range unsubIndex {
		unsubIndex[i] = i
	}
	rand.Shuffle(len(unsubIndex), func(i, j int) {
		unsubIndex[i], unsubIndex[j] = unsubIndex[j], unsubIndex[i]
	})

	b.ResetTimer()
	for i := 0; i < b.N/numBatchReceivers; i++ {
		for j := 0; j < numBatchReceivers; j++ {
			ids[j] = br.SubscribeEvents(ws[j].Receiver(), 1)
		}
		for j := 0; j < numBatchReceivers; j++ {
			br.UnsubscribeEvents(ids[unsubIndex[j]])
		}
	}
}

func BenchmarkMapSubscribeUnsubscribeBatch(b *testing.B) {
	var mu sync.Mutex
	m := make(map[*Receiver]Set)
	ws := make([]Waiter, numBatchReceivers)
	for i := range ws {
		ws[i].Init()
	}

	// Generate a random order for unsubscriptions.
	unsubIndex := make([]int, numBatchReceivers)
	for i := range unsubIndex {
		unsubIndex[i] = i
	}
	rand.Shuffle(len(unsubIndex), func(i, j int) {
		unsubIndex[i], unsubIndex[j] = unsubIndex[j], unsubIndex[i]
	})

	b.ResetTimer()
	for i := 0; i < b.N/numBatchReceivers; i++ {
		for j := 0; j < numBatchReceivers; j++ {
			mu.Lock()
			m[ws[j].Receiver()] = Set(1)
			mu.Unlock()
		}
		for j := 0; j < numBatchReceivers; j++ {
			mu.Lock()
			delete(m, ws[unsubIndex[j]].Receiver())
			mu.Unlock()
		}
	}
}

func BenchmarkQueueSubscribeUnsubscribeBatch(b *testing.B) {
	var q waiter.Queue
	es := make([]waiter.Entry, numBatchReceivers)
	for i := range es {
		es[i], _ = waiter.NewChannelEntry(nil)
	}

	// Generate a random order for unsubscriptions.
	unsubIndex := make([]int, numBatchReceivers)
	for i := range unsubIndex {
		unsubIndex[i] = i
	}
	rand.Shuffle(len(unsubIndex), func(i, j int) {
		unsubIndex[i], unsubIndex[j] = unsubIndex[j], unsubIndex[i]
	})

	b.ResetTimer()
	for i := 0; i < b.N/numBatchReceivers; i++ {
		for j := 0; j < numBatchReceivers; j++ {
			q.EventRegister(&es[j], 1)
		}
		for j := 0; j < numBatchReceivers; j++ {
			q.EventUnregister(&es[unsubIndex[j]])
		}
	}
}

// Benchmark*BroadcastRedundant measures how long it takes to Broadcast
// already-pending events to multiple Receivers.

func BenchmarkBroadcasterBroadcastRedundant(b *testing.B) {
	for _, n := range receiverCountsIncludingZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var br Broadcaster
			ws := make([]Waiter, n)
			for i := range ws {
				ws[i].Init()
				br.SubscribeEvents(ws[i].Receiver(), 1)
			}
			br.Broadcast(1)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				br.Broadcast(1)
			}
		})
	}
}

func BenchmarkMapBroadcastRedundant(b *testing.B) {
	for _, n := range receiverCountsIncludingZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var mu sync.Mutex
			m := make(map[*Receiver]Set)
			ws := make([]Waiter, n)
			for i := range ws {
				ws[i].Init()
				m[ws[i].Receiver()] = Set(1)
			}
			mu.Lock()
			for r := range m {
				r.Notify(1)
			}
			mu.Unlock()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mu.Lock()
				for r := range m {
					r.Notify(1)
				}
				mu.Unlock()
			}
		})
	}
}

func BenchmarkQueueBroadcastRedundant(b *testing.B) {
	for _, n := range receiverCountsIncludingZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var q waiter.Queue
			for i := 0; i < n; i++ {
				e, _ := waiter.NewChannelEntry(nil)
				q.EventRegister(&e, 1)
			}
			q.Notify(1)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.Notify(1)
			}
		})
	}
}

// Benchmark*BroadcastAck measures how long it takes to Broadcast events to
// multiple Receivers, check that all Receivers have received the event, and
// clear the event from all Receivers.

func BenchmarkBroadcasterBroadcastAck(b *testing.B) {
	for _, n := range receiverCountsNonZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var br Broadcaster
			ws := make([]Waiter, n)
			for i := range ws {
				ws[i].Init()
				br.SubscribeEvents(ws[i].Receiver(), 1)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				br.Broadcast(1)
				for j := range ws {
					if got, want := ws[j].Pending(), Set(1); got != want {
						b.Fatalf("Receiver.Pending(): got %#x, wanted %#x", got, want)
					}
					ws[j].Ack(AllEvents)
				}
			}
		})
	}
}

func BenchmarkMapBroadcastAck(b *testing.B) {
	for _, n := range receiverCountsNonZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var mu sync.Mutex
			m := make(map[*Receiver]Set)
			ws := make([]Waiter, n)
			for i := range ws {
				ws[i].Init()
				m[ws[i].Receiver()] = Set(1)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mu.Lock()
				for r := range m {
					r.Notify(1)
				}
				mu.Unlock()
				for j := range ws {
					if got, want := ws[j].Pending(), Set(1); got != want {
						b.Fatalf("Receiver.Pending(): got %#x, wanted %#x", got, want)
					}
					ws[j].Ack(AllEvents)
				}
			}
		})
	}
}

func BenchmarkQueueBroadcastAck(b *testing.B) {
	for _, n := range receiverCountsNonZero {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			var q waiter.Queue
			chs := make([]chan struct{}, n)
			for i := range chs {
				e, ch := waiter.NewChannelEntry(nil)
				q.EventRegister(&e, 1)
				chs[i] = ch
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.Notify(1)
				for _, ch := range chs {
					select {
					case <-ch:
					default:
						b.Fatalf("channel did not receive event")
					}
				}
			}
		})
	}
}
