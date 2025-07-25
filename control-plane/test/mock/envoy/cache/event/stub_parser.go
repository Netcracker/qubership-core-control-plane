// Code generated by MockGen. DO NOT EDIT.
// Source: ../../envoy/cache/event/parser.go

// Package mock_event is a generated GoMock package.
package mock_event

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	action "github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
	events "github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
)

// MockChangeEventParser is a mock of ChangeEventParser interface.
type MockChangeEventParser struct {
	ctrl     *gomock.Controller
	recorder *MockChangeEventParserMockRecorder
}

// MockChangeEventParserMockRecorder is the mock recorder for MockChangeEventParser.
type MockChangeEventParserMockRecorder struct {
	mock *MockChangeEventParser
}

// NewMockChangeEventParser creates a new mock instance.
func NewMockChangeEventParser(ctrl *gomock.Controller) *MockChangeEventParser {
	mock := &MockChangeEventParser{ctrl: ctrl}
	mock.recorder = &MockChangeEventParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockChangeEventParser) EXPECT() *MockChangeEventParserMockRecorder {
	return m.recorder
}

// ParseChangeEvent mocks base method.
func (m *MockChangeEventParser) ParseChangeEvent(changeEvent *events.ChangeEvent) action.ActionsMap {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseChangeEvent", changeEvent)
	ret0, _ := ret[0].(action.ActionsMap)
	return ret0
}

// ParseChangeEvent indicates an expected call of ParseChangeEvent.
func (mr *MockChangeEventParserMockRecorder) ParseChangeEvent(changeEvent interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseChangeEvent", reflect.TypeOf((*MockChangeEventParser)(nil).ParseChangeEvent), changeEvent)
}

// ParseMultipleChangeEvent mocks base method.
func (m *MockChangeEventParser) ParseMultipleChangeEvent(changeEvent *events.MultipleChangeEvent) map[string]action.SnapshotUpdateAction {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseMultipleChangeEvent", changeEvent)
	ret0, _ := ret[0].(map[string]action.SnapshotUpdateAction)
	return ret0
}

// ParseMultipleChangeEvent indicates an expected call of ParseMultipleChangeEvent.
func (mr *MockChangeEventParserMockRecorder) ParseMultipleChangeEvent(changeEvent interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseMultipleChangeEvent", reflect.TypeOf((*MockChangeEventParser)(nil).ParseMultipleChangeEvent), changeEvent)
}

// ParsePartialReloadEvent mocks base method.
func (m *MockChangeEventParser) ParsePartialReloadEvent(changeEvent *events.PartialReloadEvent) map[string]action.SnapshotUpdateAction {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParsePartialReloadEvent", changeEvent)
	ret0, _ := ret[0].(map[string]action.SnapshotUpdateAction)
	return ret0
}

// ParsePartialReloadEvent indicates an expected call of ParsePartialReloadEvent.
func (mr *MockChangeEventParserMockRecorder) ParsePartialReloadEvent(changeEvent interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParsePartialReloadEvent", reflect.TypeOf((*MockChangeEventParser)(nil).ParsePartialReloadEvent), changeEvent)
}
