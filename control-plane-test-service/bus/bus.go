package bus

import (
	"sync"
	"sync/atomic"

	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var log = logging.GetLogger("bus")

const (
	TopicChanges         = "changes"
	TopicMultipleChanges = "multiple-change"
	TopicReload          = "reload"
	Test                 = "test"

	EventBusPort = 5431 // port for event base that is used for communication between CP nodes
)

// BusPublisher defines contract for event bus publisher (master).
type BusPublisher interface {
	Publish(topic string, data interface{}) error
	Shutdown()
}

// BusSubscriber defines contract for event bus subscriber (slave).
type BusSubscriber interface {
	Subscribe(topic string, handler func(data interface{}))
	Shutdown()
}

// SubscribersCache is a thread-safe cache for holding any representation of subscriptions
// (event handler functions, go channels for messages, etc) grouped by topics.
type SubscribersCache struct {
	topicSubs *sync.Map
	size      int32
}

func NewSubscribersCache() *SubscribersCache {
	return &SubscribersCache{topicSubs: &sync.Map{}}
}

func (cache *SubscribersCache) ForEach(f func(topic string, subs []interface{}) bool) {
	cache.topicSubs.Range(func(topicName, topicData interface{}) bool {
		cachedTopic := topicData.(*topic)
		cachedTopic.rlock()
		defer cachedTopic.runlock()

		return f(topicName.(string), cachedTopic.subs)
	})
}

func (cache *SubscribersCache) ForEachSubInTopic(topic string, f func(sub interface{}) bool) {
	if cachedTopic, exists := cache.getTopic(topic); exists {
		cachedTopic.rlock()
		defer cachedTopic.runlock()

		for _, sub := range cachedTopic.subs {
			if !f(sub) {
				return
			}
		}
	}
}

func (cache *SubscribersCache) Add(topic string, sub interface{}) {
	cachedTopic := cache.getOrCreateTopic(topic)
	cachedTopic.lock()
	defer cachedTopic.unlock()
	cachedTopic.subs = append(cachedTopic.subs, sub)
	atomic.StoreInt32(&cache.size, cache.size+1)
}

func (cache *SubscribersCache) Remove(topic string, sub interface{}) {
	if cachedTopic, present := cache.getTopic(topic); present {
		cachedTopic.lock()
		defer cachedTopic.unlock()

		if len(cachedTopic.subs) > 0 {
			cachedTopic.removeSub(sub)
		}
		if len(cachedTopic.subs) == 0 {
			cache.topicSubs.Delete(topic)
		}
		atomic.StoreInt32(&cache.size, cache.size-1)
	}
}

func (cache *SubscribersCache) getTopic(name string) (*topic, bool) {
	res, found := cache.topicSubs.Load(name)
	if found {
		return res.(*topic), found
	} else {
		return nil, false
	}
}

func (cache *SubscribersCache) getOrCreateTopic(name string) *topic {
	res, _ := cache.topicSubs.LoadOrStore(name, newTopic())
	return res.(*topic)
}

// topic represents subscriptions on topic. This internal API that must be used only by SubscribersCache internal logic.
type topic struct {
	// subs holds slice of subscriptions of any type (can be handler functions, go channels, etc).
	subs []interface{}
	// mutex is RWMutex that is used for synchronization on this specific topic.
	mutex *sync.RWMutex
}

func newTopic() *topic {
	return &topic{subs: make([]interface{}, 0), mutex: &sync.RWMutex{}}
}

func (topic *topic) removeSub(sub interface{}) {
	newList := topic.subs
	i := 0 // output index for rewritten slice
	for _, existingSub := range newList {
		if existingSub != sub {
			// copy and increment index
			newList[i] = existingSub
			i++
		}
	}
	topic.subs = newList[:i]
}

func (topic *topic) lock() {
	topic.mutex.Lock()
}

func (topic *topic) unlock() {
	topic.mutex.Unlock()
}

func (topic *topic) rlock() {
	topic.mutex.RLock()
}

func (topic *topic) runlock() {
	topic.mutex.RUnlock()
}
