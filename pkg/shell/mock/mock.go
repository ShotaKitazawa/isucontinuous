// Code generated by MockGen. DO NOT EDIT.
// Source: ./common.go

// Package mock is a generated GoMock package.
package mock

import (
	bytes "bytes"
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockIface is a mock of Iface interface.
type MockIface struct {
	ctrl     *gomock.Controller
	recorder *MockIfaceMockRecorder
}

// MockIfaceMockRecorder is the mock recorder for MockIface.
type MockIfaceMockRecorder struct {
	mock *MockIface
}

// NewMockIface creates a new mock instance.
func NewMockIface(ctrl *gomock.Controller) *MockIface {
	mock := &MockIface{ctrl: ctrl}
	mock.recorder = &MockIfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIface) EXPECT() *MockIfaceMockRecorder {
	return m.recorder
}

// Deploy mocks base method.
func (m *MockIface) Deploy(ctx context.Context, src, dst string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", ctx, src, dst)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockIfaceMockRecorder) Deploy(ctx, src, dst interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockIface)(nil).Deploy), ctx, src, dst)
}

// Exec mocks base method.
func (m *MockIface) Exec(ctx context.Context, basedir, command string) (bytes.Buffer, bytes.Buffer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exec", ctx, basedir, command)
	ret0, _ := ret[0].(bytes.Buffer)
	ret1, _ := ret[1].(bytes.Buffer)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Exec indicates an expected call of Exec.
func (mr *MockIfaceMockRecorder) Exec(ctx, basedir, command interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockIface)(nil).Exec), ctx, basedir, command)
}

// Execf mocks base method.
func (m *MockIface) Execf(ctx context.Context, basedir, command string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, basedir, command}
	for _, a_2 := range a {
		varargs = append(varargs, a_2)
	}
	ret := m.ctrl.Call(m, "Execf", varargs...)
	ret0, _ := ret[0].(bytes.Buffer)
	ret1, _ := ret[1].(bytes.Buffer)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Execf indicates an expected call of Execf.
func (mr *MockIfaceMockRecorder) Execf(ctx, basedir, command interface{}, a ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, basedir, command}, a...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execf", reflect.TypeOf((*MockIface)(nil).Execf), varargs...)
}

// Host mocks base method.
func (m *MockIface) Host() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Host")
	ret0, _ := ret[0].(string)
	return ret0
}

// Host indicates an expected call of Host.
func (mr *MockIfaceMockRecorder) Host() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Host", reflect.TypeOf((*MockIface)(nil).Host))
}