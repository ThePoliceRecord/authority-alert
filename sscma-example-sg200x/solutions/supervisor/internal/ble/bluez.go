package ble

import (
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// D-Bus object paths used for the GATT application.
const (
	basePath    = "/com/recamera/ble"
	servicePath = basePath + "/service0"
	advertPath  = basePath + "/advert0"
	adapterPath = "/org/bluez/hci0"
)

// charPath returns the D-Bus object path for characteristic index i.
func charPath(i int) dbus.ObjectPath {
	return dbus.ObjectPath(servicePath + "/char" + string(rune('0'+i)))
}

// ---- GattApplication (org.freedesktop.DBus.ObjectManager) ----

// GattApplication implements the ObjectManager interface that BlueZ calls
// after RegisterApplication to discover the GATT structure.
type GattApplication struct {
	objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
}

// GetManagedObjects returns all managed GATT objects and their properties.
func (a *GattApplication) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	return a.objects, nil
}

// ---- GattService (org.bluez.GattService1) ----

// GattService represents a GATT service exported over D-Bus.
type GattService struct {
	props map[string]dbus.Variant
}

// GetAll returns all properties for the given interface.
func (s *GattService) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.GattService1" {
		return s.props, nil
	}
	return nil, nil
}

// Get returns a single property.
func (s *GattService) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.GattService1" {
		if v, ok := s.props[prop]; ok {
			return v, nil
		}
	}
	return dbus.Variant{}, nil
}

// Set is a no-op (properties are read-only).
func (s *GattService) Set(iface, prop string, val dbus.Variant) *dbus.Error { return nil }

// ---- GattCharacteristic (org.bluez.GattCharacteristic1) ----

// GattCharacteristic represents a GATT characteristic exported over D-Bus.
type GattCharacteristic struct {
	Path      dbus.ObjectPath
	props     map[string]dbus.Variant
	notifying bool
	onRead    func() ([]byte, *dbus.Error)
	onWrite   func([]byte) *dbus.Error
}

// ReadValue is called by BlueZ when a central reads this characteristic.
func (c *GattCharacteristic) ReadValue(options map[string]dbus.Variant) ([]byte, *dbus.Error) {
	if c.onRead != nil {
		return c.onRead()
	}
	return nil, nil
}

// WriteValue is called by BlueZ when a central writes to this characteristic.
func (c *GattCharacteristic) WriteValue(value []byte, options map[string]dbus.Variant) *dbus.Error {
	if c.onWrite != nil {
		return c.onWrite(value)
	}
	return nil
}

// StartNotify is called by BlueZ when a central subscribes to notifications.
func (c *GattCharacteristic) StartNotify() *dbus.Error {
	c.notifying = true
	return nil
}

// StopNotify is called by BlueZ when a central unsubscribes from notifications.
func (c *GattCharacteristic) StopNotify() *dbus.Error {
	c.notifying = false
	return nil
}

// GetAll returns all properties for the given interface.
func (c *GattCharacteristic) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.GattCharacteristic1" {
		return c.props, nil
	}
	return nil, nil
}

// Get returns a single property.
func (c *GattCharacteristic) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.GattCharacteristic1" {
		if v, ok := c.props[prop]; ok {
			return v, nil
		}
	}
	return dbus.Variant{}, nil
}

// Set is a no-op (properties are read-only).
func (c *GattCharacteristic) Set(iface, prop string, val dbus.Variant) *dbus.Error { return nil }

// sendNotify emits a PropertiesChanged signal with the new Value.
func (c *GattCharacteristic) sendNotify(conn *dbus.Conn, value []byte) error {
	return conn.Emit(c.Path,
		"org.freedesktop.DBus.Properties.PropertiesChanged",
		"org.bluez.GattCharacteristic1",
		map[string]dbus.Variant{"Value": dbus.MakeVariant(value)},
		[]string{},
	)
}

// ---- LEAdvertisement (org.bluez.LEAdvertisement1) ----

// LEAdvertisement represents a BLE advertisement exported over D-Bus.
type LEAdvertisement struct {
	props map[string]dbus.Variant
}

// GetAll returns all properties for the given interface.
func (a *LEAdvertisement) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.LEAdvertisement1" {
		return a.props, nil
	}
	return nil, nil
}

// Get returns a single property.
func (a *LEAdvertisement) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface == "org.bluez.LEAdvertisement1" {
		if v, ok := a.props[prop]; ok {
			return v, nil
		}
	}
	return dbus.Variant{}, nil
}

// Set is a no-op.
func (a *LEAdvertisement) Set(iface, prop string, val dbus.Variant) *dbus.Error { return nil }

// Release is called by BlueZ when the advertisement is unregistered.
func (a *LEAdvertisement) Release() *dbus.Error { return nil }

// ---- Builder helpers ----

// newCharacteristic creates a GattCharacteristic with the given UUID and flags.
func newCharacteristic(idx int, uuid string, flags []string) *GattCharacteristic {
	path := charPath(idx)
	return &GattCharacteristic{
		Path: path,
		props: map[string]dbus.Variant{
			"UUID":    dbus.MakeVariant(uuid),
			"Service": dbus.MakeVariant(dbus.ObjectPath(servicePath)),
			"Flags":   dbus.MakeVariant(flags),
		},
	}
}

// ---- D-Bus export / registration ----

// exportObjects exports all GATT objects onto the D-Bus connection and returns
// the five characteristic objects (DeviceInfo, Session, WiFiScan, WiFiConfig, Setup).
func exportObjects(conn *dbus.Conn, advName string) ([5]*GattCharacteristic, error) {
	// Characteristics: 0=DeviceInfo(read), 1=Session(write+notify),
	// 2=WiFiScan(write+notify), 3=WiFiConfig(write+notify), 4=Setup(write+notify)
	chars := [5]*GattCharacteristic{
		newCharacteristic(0, DeviceInfoUUID, []string{"read"}),
		newCharacteristic(1, SessionUUID, []string{"write-without-response", "notify"}),
		newCharacteristic(2, WiFiScanUUID, []string{"write-without-response", "notify"}),
		newCharacteristic(3, WiFiConfigUUID, []string{"write-without-response", "notify"}),
		newCharacteristic(4, SetupUUID, []string{"write-without-response", "notify"}),
	}

	// Build char object paths for service property.
	charPaths := make([]dbus.ObjectPath, len(chars))
	for i := range chars {
		charPaths[i] = chars[i].Path
	}

	// Service
	svc := &GattService{
		props: map[string]dbus.Variant{
			"UUID":            dbus.MakeVariant(ServiceUUID),
			"Primary":         dbus.MakeVariant(true),
			"Characteristics": dbus.MakeVariant(charPaths),
		},
	}

	// Advertisement
	adv := &LEAdvertisement{
		props: map[string]dbus.Variant{
			"Type":         dbus.MakeVariant("peripheral"),
			"LocalName":    dbus.MakeVariant(advName),
			"ServiceUUIDs": dbus.MakeVariant([]string{ServiceUUID}),
			"Includes":     dbus.MakeVariant([]string{"tx-power"}),
		},
	}

	// Build ObjectManager map for the GattApplication.
	objects := make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)

	objects[dbus.ObjectPath(servicePath)] = map[string]map[string]dbus.Variant{
		"org.bluez.GattService1": svc.props,
	}
	for _, c := range chars {
		objects[c.Path] = map[string]map[string]dbus.Variant{
			"org.bluez.GattCharacteristic1": c.props,
		}
	}

	app := &GattApplication{objects: objects}

	// Export application (ObjectManager).
	if err := conn.Export(app, dbus.ObjectPath(basePath),
		"org.freedesktop.DBus.ObjectManager"); err != nil {
		return chars, err
	}
	// Introspectable for application root.
	introApp := introspect.NewIntrospectable(&introspect.Node{
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{Name: "org.freedesktop.DBus.ObjectManager",
				Methods: []introspect.Method{{
					Name: "GetManagedObjects",
					Args: []introspect.Arg{{
						Name:      "objects",
						Type:      "a{oa{sa{sv}}}",
						Direction: "out",
					}},
				}},
			},
		},
	})
	if err := conn.Export(introApp, dbus.ObjectPath(basePath),
		"org.freedesktop.DBus.Introspectable"); err != nil {
		return chars, err
	}

	// Export service.
	for _, iface := range []string{"org.bluez.GattService1", "org.freedesktop.DBus.Properties"} {
		if err := conn.Export(svc, dbus.ObjectPath(servicePath), iface); err != nil {
			return chars, err
		}
	}

	// Export characteristics.
	for _, c := range chars {
		for _, iface := range []string{"org.bluez.GattCharacteristic1", "org.freedesktop.DBus.Properties"} {
			if err := conn.Export(c, c.Path, iface); err != nil {
				return chars, err
			}
		}
	}

	// Export advertisement.
	for _, iface := range []string{"org.bluez.LEAdvertisement1", "org.freedesktop.DBus.Properties"} {
		if err := conn.Export(adv, dbus.ObjectPath(advertPath), iface); err != nil {
			return chars, err
		}
	}

	return chars, nil
}

// registerGATT calls org.bluez.GattManager1.RegisterApplication on hci0.
func registerGATT(conn *dbus.Conn) error {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	call := obj.Call("org.bluez.GattManager1.RegisterApplication", 0,
		dbus.ObjectPath(basePath), map[string]dbus.Variant{})
	return call.Err
}

// unregisterGATT calls org.bluez.GattManager1.UnregisterApplication on hci0.
func unregisterGATT(conn *dbus.Conn) {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	obj.Call("org.bluez.GattManager1.UnregisterApplication", 0,
		dbus.ObjectPath(basePath))
}

// registerAdvertisement calls org.bluez.LEAdvertisingManager1.RegisterAdvertisement on hci0.
func registerAdvertisement(conn *dbus.Conn) error {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	call := obj.Call("org.bluez.LEAdvertisingManager1.RegisterAdvertisement", 0,
		dbus.ObjectPath(advertPath), map[string]dbus.Variant{})
	return call.Err
}

// unregisterAdvertisement calls org.bluez.LEAdvertisingManager1.UnregisterAdvertisement.
func unregisterAdvertisement(conn *dbus.Conn) {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	obj.Call("org.bluez.LEAdvertisingManager1.UnregisterAdvertisement", 0,
		dbus.ObjectPath(advertPath))
}

// ensureAdapterPowered sets org.bluez.Adapter1.Powered = true on hci0.
func ensureAdapterPowered(conn *dbus.Conn) error {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	call := obj.Call("org.freedesktop.DBus.Properties.Set", 0,
		"org.bluez.Adapter1", "Powered", dbus.MakeVariant(true))
	return call.Err
}

// setAdapterAlias sets org.bluez.Adapter1.Alias on hci0 so that BLE clients
// that display the adapter name (rather than the advertisement LocalName)
// see the correct device name.
func setAdapterAlias(conn *dbus.Conn, alias string) error {
	obj := conn.Object("org.bluez", dbus.ObjectPath(adapterPath))
	call := obj.Call("org.freedesktop.DBus.Properties.Set", 0,
		"org.bluez.Adapter1", "Alias", dbus.MakeVariant(alias))
	return call.Err
}
