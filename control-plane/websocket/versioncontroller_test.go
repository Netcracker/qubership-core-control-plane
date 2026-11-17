package websocket

import (
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	"testing"
)

func Test_ResetConnections_IfWatcherIsNotStopped_WatcherDoCatchQuitSignal(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)

	vController := &VersionController{
		watchers: NewWatcherSet(),
		data:     dao,
	}

	watcher := newWatcher()
	watcher.quit = make(chan struct{}, 1)

	vController.getWatchers().Push(watcher)
	vController.ResetConnections(clustering.NodeInfo{}, clustering.Role(1))

	select {
	case <-watcher.quit:
	default:
		t.Fatal("watcher has not received the quit signal")
	}
}

func Test_ResetConnections_IfWatcherIsStopped_WatcherDoNotCatchQuitSignal(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)

	vController := &VersionController{
		watchers: NewWatcherSet(),
		data:     dao,
	}

	watcher := newWatcher()
	watcher.quit = make(chan struct{}, 1)

	vController.getWatchers().Push(watcher)
	watcher.Stop()
	vController.ResetConnections(clustering.NodeInfo{}, clustering.Role(1))

	select {
	case <-watcher.quit:
		t.Fatal("stopped watcher received a quit signal")
	default:
	}
}
