// Code generated by MockGen. DO NOT EDIT.
// Source: ../../envoy/cache/builder/routeconfig/virtualhost.go

// Package mock_routeconfig is a generated GoMock package.
package mock_routeconfig

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockVersionAliasesProvider is a mock of VersionAliasesProvider interface.
type MockVersionAliasesProvider struct {
	ctrl     *gomock.Controller
	recorder *MockVersionAliasesProviderMockRecorder
}

// MockVersionAliasesProviderMockRecorder is the mock recorder for MockVersionAliasesProvider.
type MockVersionAliasesProviderMockRecorder struct {
	mock *MockVersionAliasesProvider
}

// NewMockVersionAliasesProvider creates a new mock instance.
func NewMockVersionAliasesProvider(ctrl *gomock.Controller) *MockVersionAliasesProvider {
	mock := &MockVersionAliasesProvider{ctrl: ctrl}
	mock.recorder = &MockVersionAliasesProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVersionAliasesProvider) EXPECT() *MockVersionAliasesProviderMockRecorder {
	return m.recorder
}

// GetVersionAliases mocks base method.
func (m *MockVersionAliasesProvider) GetVersionAliases() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVersionAliases")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetVersionAliases indicates an expected call of GetVersionAliases.
func (mr *MockVersionAliasesProviderMockRecorder) GetVersionAliases() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVersionAliases", reflect.TypeOf((*MockVersionAliasesProvider)(nil).GetVersionAliases))
}
