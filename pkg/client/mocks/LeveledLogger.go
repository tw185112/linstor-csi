// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// LeveledLogger is an autogenerated mock type for the LeveledLogger type
type LeveledLogger struct {
	mock.Mock
}

// Debugf provides a mock function with given fields: _a0, _a1
func (_m *LeveledLogger) Debugf(_a0 string, _a1 ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _a1...)
	_m.Called(_ca...)
}

// Errorf provides a mock function with given fields: _a0, _a1
func (_m *LeveledLogger) Errorf(_a0 string, _a1 ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _a1...)
	_m.Called(_ca...)
}

// Infof provides a mock function with given fields: _a0, _a1
func (_m *LeveledLogger) Infof(_a0 string, _a1 ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _a1...)
	_m.Called(_ca...)
}

// Warnf provides a mock function with given fields: _a0, _a1
func (_m *LeveledLogger) Warnf(_a0 string, _a1 ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _a1...)
	_m.Called(_ca...)
}
