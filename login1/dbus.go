// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package login1 provides integration with the systemd logind API.  See http://www.freedesktop.org/wiki/Software/systemd/logind/
package login1

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	dbusDest             = "org.freedesktop.login1"
	dbusManagerInterface = "org.freedesktop.login1.Manager"
	dbusSessionInterface = "org.freedesktop.login1.Session"
	dbusUserInterface    = "org.freedesktop.login1.User"
	dbusPath             = "/org/freedesktop/login1"
)

// Conn is a connection to systemds dbus endpoint.
type Conn struct {
	conn   *dbus.Conn
	object dbus.BusObject
}

// New establishes a connection to the system bus and authenticates.
func New() (*Conn, error) {
	c := new(Conn)

	if err := c.initConnection(); err != nil {
		return nil, err
	}

	return c, nil
}

// Close closes the dbus connection
func (c *Conn) Close() {
	if c == nil {
		return
	}

	if c.conn != nil {
		c.conn.Close()
	}
}

// Connected returns whether conn is connected
func (c *Conn) Connected() bool {
	return c.conn.Connected()
}

func (c *Conn) initConnection() error {
	var err error
	c.conn, err = dbus.SystemBusPrivate()
	if err != nil {
		return err
	}

	// Only use EXTERNAL method, and hardcode the uid (not username)
	// to avoid a username lookup (which requires a dynamically linked
	// libc)
	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Getuid()))}

	err = c.conn.Auth(methods)
	if err != nil {
		c.conn.Close()
		return err
	}

	err = c.conn.Hello()
	if err != nil {
		c.conn.Close()
		return err
	}

	c.object = c.conn.Object("org.freedesktop.login1", dbus.ObjectPath(dbusPath))

	return nil
}

// Session object definition.
type Session struct {
	ID   string
	UID  uint32
	User string
	Seat string
	Path dbus.ObjectPath
}

// User object definition.
type User struct {
	UID  uint32
	Name string
	Path dbus.ObjectPath
}

func (s Session) toInterface() []interface{} {
	return []interface{}{s.ID, s.UID, s.User, s.Seat, s.Path}
}

func sessionFromInterfaces(session []interface{}) (*Session, error) {
	if len(session) < 5 {
		return nil, fmt.Errorf("invalid number of session fields: %d", len(session))
	}
	id, ok := session[0].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 0 to string")
	}
	uid, ok := session[1].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 1 to uint32")
	}
	user, ok := session[2].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 2 to string")
	}
	seat, ok := session[3].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 2 to string")
	}
	path, ok := session[4].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 4 to ObjectPath")
	}

	ret := Session{ID: id, UID: uid, User: user, Seat: seat, Path: path}
	return &ret, nil
}

func userFromInterfaces(user []interface{}) (*User, error) {
	if len(user) < 3 {
		return nil, fmt.Errorf("invalid number of user fields: %d", len(user))
	}
	uid, ok := user[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 0 to uint32")
	}
	name, ok := user[1].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 1 to string")
	}
	path, ok := user[2].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 2 to ObjectPath")
	}

	ret := User{UID: uid, Name: name, Path: path}
	return &ret, nil
}

// GetActiveSession may be used to get the session object path for the current active session
func (c *Conn) GetActiveSession() (dbus.ObjectPath, error) {
	var seat0Path dbus.ObjectPath
	if err := c.object.Call(dbusManagerInterface+".GetSeat", 0, "seat0").Store(&seat0Path); err != nil {
		return "", err
	}

	seat0Obj := c.conn.Object(dbusDest, seat0Path)
	activeSession, err := seat0Obj.GetProperty(dbusDest + ".Seat.ActiveSession")
	if err != nil {
		return "", err
	}
	activeSessionMap, ok := activeSession.Value().([]interface{})
	if !ok || len(activeSessionMap) < 2 {
		return "", fmt.Errorf("failed to typecast active session map")
	}

	activeSessionPath, ok := activeSessionMap[1].(dbus.ObjectPath)
	if !ok {
		return "", fmt.Errorf("failed to typecast dbus active session Path")
	}
	return activeSessionPath, nil
}

// GetSessionUser may be used to get the user of specific session
func (c *Conn) GetSessionUser(sessionPath dbus.ObjectPath) (*User, error) {
	if len(sessionPath) == 0 {
		return nil, fmt.Errorf("empty sessionPath")
	}

	activeSessionObj := c.conn.Object(dbusDest, sessionPath)
	sessionUserName, err := activeSessionObj.GetProperty(dbusDest + ".Session.Name")
	if err != nil {
		return nil, err
	}

	sessionUser, err := activeSessionObj.GetProperty(dbusDest + ".Session.User")
	if err != nil {
		return nil, err
	}
	dbusUser, ok := sessionUser.Value().([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to typecast dbus session user")
	}

	if len(dbusUser) < 2 {
		return nil, fmt.Errorf("invalid number of user fields: %d", len(dbusUser))
	}
	uid, ok := dbusUser[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 0 to uint32")
	}
	path, ok := dbusUser[1].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 1 to ObjectPath")
	}

	user := User{UID: uid, Name: strings.Trim(sessionUserName.String(), "\""), Path: path}

	return &user, nil
}

// GetSessionDisplay may be used to get the display for specific session
func (c *Conn) GetSessionDisplay(sessionPath dbus.ObjectPath) (string, error) {
	if len(sessionPath) == 0 {
		return "", fmt.Errorf("empty sessionPath")
	}
	sessionObj := c.conn.Object(dbusDest, sessionPath)
	display, err := sessionObj.GetProperty(dbusDest + ".Session.Display")
	if err != nil {
		return "", err
	}

	return strings.Trim(display.String(), "\""), nil
}

// GetSession may be used to get the session object path for the session with the specified ID.
func (c *Conn) GetSession(id string) (dbus.ObjectPath, error) {
	var out interface{}
	if err := c.object.Call(dbusManagerInterface+".GetSession", 0, id).Store(&out); err != nil {
		return "", err
	}

	ret, ok := out.(dbus.ObjectPath)
	if !ok {
		return "", fmt.Errorf("failed to typecast session to ObjectPath")
	}

	return ret, nil
}

// Deprecated: use ListSessionsContext instead.
func (c *Conn) ListSessions() ([]Session, error) {
	return c.ListSessionsContext(context.Background())
}

// ListSessionsContext returns an array with all current sessions.
func (c *Conn) ListSessionsContext(ctx context.Context) ([]Session, error) {
	out := [][]interface{}{}
	if err := c.object.CallWithContext(ctx, dbusManagerInterface+".ListSessions", 0).Store(&out); err != nil {
		return nil, err
	}

	ret := []Session{}
	for _, el := range out {
		session, err := sessionFromInterfaces(el)
		if err != nil {
			return nil, err
		}
		ret = append(ret, *session)
	}
	return ret, nil
}

// Deprecated: use ListUsersContext instead.
func (c *Conn) ListUsers() ([]User, error) {
	return c.ListUsersContext(context.Background())
}

// ListUsersContext returns an array with all currently logged-in users.
func (c *Conn) ListUsersContext(ctx context.Context) ([]User, error) {
	out := [][]interface{}{}
	if err := c.object.CallWithContext(ctx, dbusManagerInterface+".ListUsers", 0).Store(&out); err != nil {
		return nil, err
	}

	ret := []User{}
	for _, el := range out {
		user, err := userFromInterfaces(el)
		if err != nil {
			return nil, err
		}
		ret = append(ret, *user)
	}
	return ret, nil
}

// GetSessionPropertiesContext takes a session path and returns all of its dbus object properties.
func (c *Conn) GetSessionPropertiesContext(ctx context.Context, sessionPath dbus.ObjectPath) (map[string]dbus.Variant, error) {
	return c.getProperties(ctx, sessionPath, dbusSessionInterface)
}

// GetSessionPropertyContext takes a session path and a property name and returns the property value.
func (c *Conn) GetSessionPropertyContext(ctx context.Context, sessionPath dbus.ObjectPath, property string) (*dbus.Variant, error) {
	return c.getProperty(ctx, sessionPath, dbusSessionInterface, property)
}

// GetUserPropertiesContext takes a user path and returns all of its dbus object properties.
func (c *Conn) GetUserPropertiesContext(ctx context.Context, userPath dbus.ObjectPath) (map[string]dbus.Variant, error) {
	return c.getProperties(ctx, userPath, dbusUserInterface)
}

// GetUserPropertyContext takes a user path and a property name and returns the property value.
func (c *Conn) GetUserPropertyContext(ctx context.Context, userPath dbus.ObjectPath, property string) (*dbus.Variant, error) {
	return c.getProperty(ctx, userPath, dbusUserInterface, property)
}

// LockSession asks the session with the specified ID to activate the screen lock.
func (c *Conn) LockSession(id string) {
	c.object.Call(dbusManagerInterface+".LockSession", 0, id)
}

// LockSessions asks all sessions to activate the screen locks. This may be used to lock any access to the machine in one action.
func (c *Conn) LockSessions() {
	c.object.Call(dbusManagerInterface+".LockSessions", 0)
}

// TerminateSession forcibly terminate one specific session.
func (c *Conn) TerminateSession(id string) {
	c.object.Call(dbusManagerInterface+".TerminateSession", 0, id)
}

// TerminateUser forcibly terminates all processes of a user.
func (c *Conn) TerminateUser(uid uint32) {
	c.object.Call(dbusManagerInterface+".TerminateUser", 0, uid)
}

// Reboot asks logind for a reboot optionally asking for auth.
func (c *Conn) Reboot(askForAuth bool) {
	c.object.Call(dbusManagerInterface+".Reboot", 0, askForAuth)
}

// Inhibit takes inhibition lock in logind.
func (c *Conn) Inhibit(what, who, why, mode string) (*os.File, error) {
	var fd dbus.UnixFD

	err := c.object.Call(dbusManagerInterface+".Inhibit", 0, what, who, why, mode).Store(&fd)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), "inhibit"), nil
}

// Subscribe to signals on the logind dbus
func (c *Conn) Subscribe(members ...string) chan *dbus.Signal {
	for _, member := range members {
		c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
			fmt.Sprintf("type='signal',interface='org.freedesktop.login1.Manager',member='%s'", member))
	}
	ch := make(chan *dbus.Signal, 10)
	c.conn.Signal(ch)
	return ch
}

// PowerOff asks logind for a power off optionally asking for auth.
func (c *Conn) PowerOff(askForAuth bool) {
	c.object.Call(dbusManagerInterface+".PowerOff", 0, askForAuth)
}

func (c *Conn) getProperties(ctx context.Context, path dbus.ObjectPath, dbusInterface string) (map[string]dbus.Variant, error) {
	if !path.IsValid() {
		return nil, fmt.Errorf("invalid object path (%s)", path)
	}

	obj := c.conn.Object(dbusDest, path)

	var props map[string]dbus.Variant
	err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, dbusInterface).Store(&props)
	if err != nil {
		return nil, err
	}

	return props, nil
}

func (c *Conn) getProperty(ctx context.Context, path dbus.ObjectPath, dbusInterface, property string) (*dbus.Variant, error) {
	if !path.IsValid() {
		return nil, fmt.Errorf("invalid object path (%s)", path)
	}

	obj := c.conn.Object(dbusDest, path)

	var prop dbus.Variant
	err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, dbusInterface, property).Store(&prop)
	if err != nil {
		return nil, err
	}

	return &prop, nil
}
