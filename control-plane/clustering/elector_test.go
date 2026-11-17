package clustering

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type TestEnvironment struct {
	electionService  *ElectionServiceMock
	lifeCycleManager *LifeCycleManager
	electorConfig    *ElectorConfig
}

func initElectorTestEnvironment() (TestEnvironment, error) {
	electionService := NewElectionServiceMock()
	err := electionService.tryAddInternalRecord()
	if err != nil {
		return TestEnvironment{}, err
	}

	lifeCycleManager := NewLifeCycleManager(
		"test-node",
		"default",
		NodeInfo{
			IP:       "0.0.0.0",
			SWIMPort: 0,
			BusPort:  0,
			HttpPort: 8080,
		},
		MasterNodeInitializerMock{},
	)

	electorConfig := ElectorConfig{
		ElectionService:     electionService,
		LifeCycleManager:    lifeCycleManager,
		MasterLoopSleepTime: 10 * time.Millisecond,
		SlaveLoopSleepTime:  10 * time.Millisecond,
		SyncClockTimeShift:  1 * time.Second,
	}

	return TestEnvironment{
		electionService:  electionService,
		lifeCycleManager: lifeCycleManager,
		electorConfig:    &electorConfig,
	}, nil
}

func TestElector_ElectorCreation(t *testing.T) {
	electorConfig := ElectorConfig{}
	_, err := NewElector(electorConfig)
	assert.NotNil(t, err)
	electionService := NewElectionServiceMock()
	electorConfig = ElectorConfig{
		ElectionService: electionService,
	}
	_, err = NewElector(electorConfig)
	assert.NotNil(t, err)
	lifeCycleManager := NewLifeCycleManager("", "", NodeInfo{}, nil)
	electorConfig = ElectorConfig{
		ElectionService:  electionService,
		LifeCycleManager: lifeCycleManager,
	}
	elector, err := NewElector(electorConfig)
	assert.Nil(t, err)
	assert.NotNil(t, elector)
}

func TestElector_TestMasterElection(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	require.NoError(t, err)
	testEnv.lifeCycleManager.thisNode.Name = "master-node"
	elector, err := NewElector(*testEnv.electorConfig)
	require.NoError(t, err)

	runElectionUntilRole(t, elector, Master)
	assert.Equal(t, Master, CurrentNodeState.GetRole())
}

func TestElector_TestSlaveElection(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	require.NoError(t, err)
	testEnv.lifeCycleManager.thisNode.Name = "master-node"
	elector, err := NewElector(*testEnv.electorConfig)
	require.NoError(t, err)

	elector.config.ElectionService.TryWriteAsMaster(&elector.record)
	testEnv.lifeCycleManager.thisNode.Name = "slave-node"
	elector.record.Name = "slave-node"

	runElectionUntilRole(t, elector, Slave)
	assert.Equal(t, Slave, CurrentNodeState.GetRole())
}

func TestElector_TestMasterElection_Compound(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	require.NoError(t, err)
	testEnv.lifeCycleManager.thisNode.Name = "master-node"
	elector, err := NewElector(*testEnv.electorConfig)
	require.NoError(t, err)
	var roles []Role
	testEnv.lifeCycleManager.AddOnRoleChanged(func(info NodeInfo, role Role) error {
		roles = append(roles, role)
		if role == Master {
			// Change identity in the election goroutine after it becomes master.
			testEnv.lifeCycleManager.thisNode.Name = "slave-node"
			elector.record.Name = "slave-node"
		}
		return nil
	})
	runElectionUntilRole(t, elector, Slave)
	assert.Equal(t, []Role{Master, Slave}, roles)
	assert.False(t, CurrentNodeState.IsMaster())
}

func TestElector_TestSlaveElection_NoRecords(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	require.NoError(t, err)
	testEnv.electionService.clearTable()
	testEnv.lifeCycleManager.thisNode.Name = "slave-node"
	elector, err := NewElector(*testEnv.electorConfig)
	require.NoError(t, err)

	runElectionUntilRole(t, elector, Phantom)
	assert.Equal(t, Phantom, CurrentNodeState.GetRole())
}

func TestElector_TestMasterProlongation(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}
	elector.config.ElectionService.TryWriteAsMaster(&elector.record)
	*elector.lifeCycleManager.currentMaster = elector.record

	masterRecord, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	masterRecordBefore := &MasterMetadata{}
	*masterRecordBefore = *masterRecord

	// The mock signals a successful renewal instead of stopping after a delay.
	testEnv.electionService.afterShift = elector.Stop
	elector.startMasterProlongation()

	masterRecordAfter, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	assert.NotEqual(t, masterRecordBefore.SyncClock, masterRecordAfter.SyncClock)
	*masterRecordBefore = *masterRecordAfter
	elector.stopped = false
}

func TestElector_TestMasterProlongation_NoRecords(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}
	elector.config.ElectionService.TryWriteAsMaster(&elector.record)
	*elector.lifeCycleManager.currentMaster = elector.record

	masterRecord, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	masterRecordBefore := &MasterMetadata{}
	*masterRecordBefore = *masterRecord

	// No master record case
	testEnv.electionService.clearTable()
	elector.startMasterProlongation()

	_, err = elector.config.ElectionService.GetMaster()
	assert.NotNil(t, err)
}

func TestElector_TestMasterProlongation_MasterHasChangedBreak(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}
	elector.config.ElectionService.TryWriteAsMaster(&elector.record)
	*elector.lifeCycleManager.currentMaster = elector.record

	masterRecord, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	masterRecordBefore := &MasterMetadata{}
	*masterRecordBefore = *masterRecord

	// Master has changed break case
	elector.config.LifeCycleManager.currentMaster.Name = "other-master-node"
	elector.startMasterProlongation()

	masterRecordAfter, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	assert.Equal(t, masterRecordBefore.SyncClock, masterRecordAfter.SyncClock)
}

func TestElector_TestMasterProlongation_ShiftSyncClockBreak(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}
	elector.config.ElectionService.TryWriteAsMaster(&elector.record)
	*elector.lifeCycleManager.currentMaster = elector.record

	masterRecord, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	masterRecordBefore := MasterMetadata{}
	masterRecordBefore = *masterRecord

	// Shift sync clock break case
	elector.config.SyncClockTimeShift = 0 * time.Second // To invoke error
	elector.startMasterProlongation()

	masterRecordAfter, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	assert.Equal(t, masterRecordBefore.SyncClock, masterRecordAfter.SyncClock)
}

func TestElector_ForceElection(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector.ticker = make(chan interface{})

	masterRecord, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	masterRecordBefore := &MasterMetadata{}
	*masterRecordBefore = *masterRecord

	done := make(chan struct{})
	go func() {
		elector.ForceElection("internal")
		close(done)
	}()
	select {
	case <-elector.ticker:
	case <-time.After(electionTestTimeout):
		t.Fatal("forced election did not signal the election loop")
	}
	select {
	case <-done:
	case <-time.After(electionTestTimeout):
		t.Fatal("forced election did not finish")
	}
	masterRecordAfter, err := elector.config.ElectionService.GetMaster()
	assert.Nil(t, err)
	assert.NotEqual(t, masterRecordBefore.SyncClock, masterRecordAfter.SyncClock)
}

func TestElector_ForceElection_NoRecords(t *testing.T) {
	testEnv, err := initElectorTestEnvironment()
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector, err := NewElector(*testEnv.electorConfig)
	if !assert.Nil(t, err) {
		assert.FailNow(t, err.Error())
	}

	elector.ticker = make(chan interface{})

	testEnv.electionService.clearTable()
	done := make(chan struct{})
	go func() {
		elector.ForceElection("internal")
		close(done)
	}()
	select {
	case <-elector.ticker:
	case <-time.After(electionTestTimeout):
		t.Fatal("forced election did not signal the election loop")
	}
	select {
	case <-done:
	case <-time.After(electionTestTimeout):
		t.Fatal("forced election did not finish")
	}
	_, err = elector.config.ElectionService.GetMaster()
	assert.NotNil(t, err)
}

type MasterNodeInitializerMock struct {
}

func (i MasterNodeInitializerMock) InitMaster() error {
	return nil
}

type ElectionServiceMock struct {
	masterMetadata []*MasterMetadata
	afterShift     func()
}

func NewElectionServiceMock() *ElectionServiceMock {
	return &ElectionServiceMock{
		masterMetadata: []*MasterMetadata{},
	}
}

func (e *ElectionServiceMock) GetMaster() (*MasterMetadata, error) {
	if len(e.masterMetadata) == 0 {
		return nil, fmt.Errorf("no rows in db")
	}
	return e.masterMetadata[0], nil
}

func (e *ElectionServiceMock) ResetSyncClock(master string) error {
	if len(e.masterMetadata) == 0 {
		return fmt.Errorf("no rows in db")
	}
	for _, metadata := range e.masterMetadata {
		if metadata.Name == master {
			metadata.SyncClock = time.Now()
		}
	}
	return nil
}

func (e *ElectionServiceMock) TryWriteAsMaster(electionRecord *MasterMetadata) bool {
	rowsAffected := 0
	curTime := time.Now()
	for _, metadata := range e.masterMetadata {
		if metadata.SyncClock.Before(curTime) {
			*metadata = *electionRecord
			metadata.SyncClock = curTime.Add(60 * time.Second)
			rowsAffected++
		}
	}
	return rowsAffected != 0
}

func (e *ElectionServiceMock) ShiftSyncClock(d time.Duration) error {
	if d < 1*time.Second {
		// Purposed only for error test
		return fmt.Errorf("duration less than a second")
	}
	if len(e.masterMetadata) == 0 {
		return fmt.Errorf("no rows in db")
	}
	e.masterMetadata[0].SyncClock = time.Now().Add(d)
	if e.afterShift != nil {
		e.afterShift()
	}
	return nil
}

func (e *ElectionServiceMock) DeleteSeveralRecordsFromDb() error {
	var newMetadata []*MasterMetadata
	for _, metadata := range e.masterMetadata {
		if metadata.Name == "internal" {
			newMetadata = append(newMetadata, metadata)
		}
	}
	e.masterMetadata = newMetadata

	//err := e.tryAddInternalRecord()
	//if err != nil {
	//	return err
	//}

	return nil
}

func (e *ElectionServiceMock) tryAddInternalRecord() error {
	count := len(e.masterMetadata)
	if count == 1 {
		return nil
	}

	if count > 1 {
		e.masterMetadata = []*MasterMetadata{}
	}

	electionRecord := MasterMetadata{
		Name: "internal",
		NodeInfo: NodeInfo{
			IP:       "0.0.0.0",
			SWIMPort: 0,
			BusPort:  0,
		},
	}
	e.masterMetadata = append(e.masterMetadata, &electionRecord)
	return nil
}

func (e *ElectionServiceMock) clearTable() {
	e.masterMetadata = make([]*MasterMetadata, 0)
}

const electionTestTimeout = 5 * time.Second

func runElectionUntilRole(t *testing.T, elector *Elector, want Role) {
	t.Helper()
	// Drive the election loop directly: no perpetual ticker goroutine is needed.
	elector.ticker = make(chan interface{}, 1)
	elector.ticker <- struct{}{}
	elector.lifeCycleManager.AddOnRoleChanged(func(_ NodeInfo, role Role) error {
		if role == want {
			elector.Stop()
		}
		return nil
	})
	done := make(chan struct{})
	go func() { elector.startElectionLoop(); close(done) }()
	select {
	case <-done:
	case <-time.After(electionTestTimeout):
		t.Fatalf("election did not reach role %v", want)
	}
}
