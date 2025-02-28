// Code generated by MockGen. DO NOT EDIT.
// Source: ./permittable.go
//
// Generated by this command:
//
//	mockgen -source=./permittable.go -destination=./mock_repo/mock_permittable.go -package mock_repo
//

// Package mock_repo is a generated GoMock package.
package mock_repo

import (
	context "context"
	reflect "reflect"

	id "github.com/reearth/reearth-accounts/pkg/id"
	permittable "github.com/reearth/reearth-accounts/pkg/permittable"
	user "github.com/reearth/reearthx/account/accountdomain/user"
	gomock "go.uber.org/mock/gomock"
)

// MockPermittable is a mock of Permittable interface.
type MockPermittable struct {
	ctrl     *gomock.Controller
	recorder *MockPermittableMockRecorder
	isgomock struct{}
}

// MockPermittableMockRecorder is the mock recorder for MockPermittable.
type MockPermittableMockRecorder struct {
	mock *MockPermittable
}

// NewMockPermittable creates a new mock instance.
func NewMockPermittable(ctrl *gomock.Controller) *MockPermittable {
	mock := &MockPermittable{ctrl: ctrl}
	mock.recorder = &MockPermittableMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPermittable) EXPECT() *MockPermittableMockRecorder {
	return m.recorder
}

// FindByRoleID mocks base method.
func (m *MockPermittable) FindByRoleID(arg0 context.Context, arg1 id.RoleID) (permittable.List, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByRoleID", arg0, arg1)
	ret0, _ := ret[0].(permittable.List)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByRoleID indicates an expected call of FindByRoleID.
func (mr *MockPermittableMockRecorder) FindByRoleID(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByRoleID", reflect.TypeOf((*MockPermittable)(nil).FindByRoleID), arg0, arg1)
}

// FindByUserID mocks base method.
func (m *MockPermittable) FindByUserID(arg0 context.Context, arg1 user.ID) (*permittable.Permittable, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByUserID", arg0, arg1)
	ret0, _ := ret[0].(*permittable.Permittable)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByUserID indicates an expected call of FindByUserID.
func (mr *MockPermittableMockRecorder) FindByUserID(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByUserID", reflect.TypeOf((*MockPermittable)(nil).FindByUserID), arg0, arg1)
}

// FindByUserIDs mocks base method.
func (m *MockPermittable) FindByUserIDs(arg0 context.Context, arg1 user.IDList) (permittable.List, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByUserIDs", arg0, arg1)
	ret0, _ := ret[0].(permittable.List)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByUserIDs indicates an expected call of FindByUserIDs.
func (mr *MockPermittableMockRecorder) FindByUserIDs(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByUserIDs", reflect.TypeOf((*MockPermittable)(nil).FindByUserIDs), arg0, arg1)
}

// Save mocks base method.
func (m *MockPermittable) Save(arg0 context.Context, arg1 permittable.Permittable) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Save indicates an expected call of Save.
func (mr *MockPermittableMockRecorder) Save(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockPermittable)(nil).Save), arg0, arg1)
}
