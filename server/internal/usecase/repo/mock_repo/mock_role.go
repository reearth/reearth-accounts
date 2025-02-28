// Code generated by MockGen. DO NOT EDIT.
// Source: ./role.go
//
// Generated by this command:
//
//	mockgen -source=./role.go -destination=./mock_repo/mock_role.go -package mock_repo
//

// Package mock_repo is a generated GoMock package.
package mock_repo

import (
	context "context"
	reflect "reflect"

	id "github.com/reearth/reearth-accounts/pkg/id"
	role "github.com/reearth/reearth-accounts/pkg/role"
	gomock "go.uber.org/mock/gomock"
)

// MockRole is a mock of Role interface.
type MockRole struct {
	ctrl     *gomock.Controller
	recorder *MockRoleMockRecorder
	isgomock struct{}
}

// MockRoleMockRecorder is the mock recorder for MockRole.
type MockRoleMockRecorder struct {
	mock *MockRole
}

// NewMockRole creates a new mock instance.
func NewMockRole(ctrl *gomock.Controller) *MockRole {
	mock := &MockRole{ctrl: ctrl}
	mock.recorder = &MockRoleMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRole) EXPECT() *MockRoleMockRecorder {
	return m.recorder
}

// FindAll mocks base method.
func (m *MockRole) FindAll(arg0 context.Context) (role.List, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindAll", arg0)
	ret0, _ := ret[0].(role.List)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindAll indicates an expected call of FindAll.
func (mr *MockRoleMockRecorder) FindAll(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindAll", reflect.TypeOf((*MockRole)(nil).FindAll), arg0)
}

// FindByID mocks base method.
func (m *MockRole) FindByID(arg0 context.Context, arg1 id.RoleID) (*role.Role, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByID", arg0, arg1)
	ret0, _ := ret[0].(*role.Role)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByID indicates an expected call of FindByID.
func (mr *MockRoleMockRecorder) FindByID(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByID", reflect.TypeOf((*MockRole)(nil).FindByID), arg0, arg1)
}

// FindByIDs mocks base method.
func (m *MockRole) FindByIDs(arg0 context.Context, arg1 id.RoleIDList) (role.List, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByIDs", arg0, arg1)
	ret0, _ := ret[0].(role.List)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByIDs indicates an expected call of FindByIDs.
func (mr *MockRoleMockRecorder) FindByIDs(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByIDs", reflect.TypeOf((*MockRole)(nil).FindByIDs), arg0, arg1)
}

// Remove mocks base method.
func (m *MockRole) Remove(arg0 context.Context, arg1 id.RoleID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockRoleMockRecorder) Remove(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockRole)(nil).Remove), arg0, arg1)
}

// Save mocks base method.
func (m *MockRole) Save(arg0 context.Context, arg1 role.Role) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Save indicates an expected call of Save.
func (mr *MockRoleMockRecorder) Save(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockRole)(nil).Save), arg0, arg1)
}
