// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import (
	context "context"

	client "github.com/LINBIT/golinstor/client"

	mock "github.com/stretchr/testify/mock"
)

// StoragePoolDefinitionProvider is an autogenerated mock type for the StoragePoolDefinitionProvider type
type StoragePoolDefinitionProvider struct {
	mock.Mock
}

// Create provides a mock function with given fields: ctx, spd
func (_m *StoragePoolDefinitionProvider) Create(ctx context.Context, spd client.StoragePoolDefinition) error {
	ret := _m.Called(ctx, spd)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, client.StoragePoolDefinition) error); ok {
		r0 = rf(ctx, spd)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Delete provides a mock function with given fields: ctx, spdName
func (_m *StoragePoolDefinitionProvider) Delete(ctx context.Context, spdName string) error {
	ret := _m.Called(ctx, spdName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, spdName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: ctx, spdName, opts
func (_m *StoragePoolDefinitionProvider) Get(ctx context.Context, spdName string, opts ...*client.ListOpts) (client.StoragePoolDefinition, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, spdName)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 client.StoragePoolDefinition
	if rf, ok := ret.Get(0).(func(context.Context, string, ...*client.ListOpts) client.StoragePoolDefinition); ok {
		r0 = rf(ctx, spdName, opts...)
	} else {
		r0 = ret.Get(0).(client.StoragePoolDefinition)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...*client.ListOpts) error); ok {
		r1 = rf(ctx, spdName, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAll provides a mock function with given fields: ctx, opts
func (_m *StoragePoolDefinitionProvider) GetAll(ctx context.Context, opts ...*client.ListOpts) ([]client.StoragePoolDefinition, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []client.StoragePoolDefinition
	if rf, ok := ret.Get(0).(func(context.Context, ...*client.ListOpts) []client.StoragePoolDefinition); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.StoragePoolDefinition)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, ...*client.ListOpts) error); ok {
		r1 = rf(ctx, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPropsInfos provides a mock function with given fields: ctx, opts
func (_m *StoragePoolDefinitionProvider) GetPropsInfos(ctx context.Context, opts ...*client.ListOpts) ([]client.PropsInfo, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []client.PropsInfo
	if rf, ok := ret.Get(0).(func(context.Context, ...*client.ListOpts) []client.PropsInfo); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.PropsInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, ...*client.ListOpts) error); ok {
		r1 = rf(ctx, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Modify provides a mock function with given fields: ctx, spdName, props
func (_m *StoragePoolDefinitionProvider) Modify(ctx context.Context, spdName string, props client.StoragePoolDefinitionModify) error {
	ret := _m.Called(ctx, spdName, props)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.StoragePoolDefinitionModify) error); ok {
		r0 = rf(ctx, spdName, props)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
