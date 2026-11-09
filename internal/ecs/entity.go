// Package ecs implements a small, custom entity-component system.
//
// # Why a custom ECS?
//
// The project forbids heavy ECS frameworks. This implementation is therefore
// handwritten and minimal, but keeps the properties that matter for a game
// loop:
//
//   - Global unique entity IDs of type uint64.
//   - Array-of-Structs (AoS) storage for each component type: components of
//     the same type live in a contiguous slice, which is cache friendly.
//   - Entities own dense index slots; component storage is sparse, so an
//     entity may carry any subset of components.
//   - Iterating a component type yields a dense slice that can be walked
//     without pointer chasing.
//
// # Concurrency
//
// The ECS itself is guarded by a RWMutex, but the intended usage is
// single-threaded access from the simulation coordinator. Parallel work is
// performed on *snapshots* of the data (see internal/world), never by
// mutating the ECS from several goroutines at once.
//
// # Limits
//
// At most 64 distinct component types may be registered; the per-entity
// component mask is a uint64 bitmask.
package ecs

import (
	"fmt"
	"reflect"
	"sync"
)

// ID is the global unique identifier of an entity.
//
// Zero is the "nil" entity and is never handed out.
type ID uint64

// MaxComponentTypes is the maximum number of distinct component types.
const MaxComponentTypes = 64

// entityRec is the per-entity bookkeeping record.
type entityRec struct {
	// alive reports whether the entity still exists.
	alive bool
	// components is the bitmask of component types attached to the entity.
	// Bit i corresponds to the storage registered at index i.
	components uint64
}

// componentBit returns the bitmask bit of a component storage index.
func componentBit(idx int) uint64 {
	return uint64(1) << uint(idx)
}

// World is the entity registry.
//
// Layout: entities are stored in a dense slice (for iteration) and a
// generation map (for O(1) id -> slot lookup).
type World struct {
	mu sync.RWMutex

	// entities is the dense entity storage; index == slot.
	entities []entityRec
	// slots maps an entity ID to its slot in the dense storage.
	slots map[ID]int
	// nextID is the next never-before-used entity ID.
	nextID ID
	// freeSlots lists slots of destroyed entities that can be recycled.
	freeSlots []int
	// components holds one storage per registered component type.
	components []ComponentStorage
	// componentIndex maps a component type name to its storage index.
	componentIndex map[string]int
}

// NewWorld creates an empty world.
func NewWorld() *World {
	return &World{
		entities: make([]entityRec, 0, 256),
		slots:    make(map[ID]int, 256),
		// The ID 0 is reserved for the "nil" entity, so the first
		// entity that is handed out has the ID 1.
		nextID:         1,
		componentIndex: make(map[string]int, 16),
	}
}

// ComponentStorage is the interface implemented by every component storage.
//
// Each concrete storage type is an Array-of-Structs: slot i of the dense
// slice corresponds to the entity whose slot index is i.
type ComponentStorage interface {
	// Len returns the number of dense slots.
	Len() int
	// ComponentID returns the component type name used for indexing.
	ComponentID() string
	// ClearSlot resets the component data of a slot (used when an entity
	// is destroyed or a component is removed).
	ClearSlot(slot int)
	// SwapSlots exchanges two dense slots.
	SwapSlots(a, b int)
	// SetSlot writes a component value into a dense slot.
	SetSlot(slot int, value any)
	// SetEntity records the owner entity of a dense slot.
	SetEntity(slot int, id ID)
}

// CreateEntity allocates a new entity and returns its ID.
//
// The returned ID is stable for the lifetime of the entity; destroying and
// recreating an entity yields a different ID.
func (w *World) CreateEntity() ID {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.createEntityLocked()
}

// createEntityLocked is the lock-free core of CreateEntity.
func (w *World) createEntityLocked() ID {
	var slot int
	if n := len(w.freeSlots); n > 0 {
		slot = w.freeSlots[n-1]
		w.freeSlots = w.freeSlots[:n-1]
		w.entities[slot] = entityRec{alive: true}
	} else {
		slot = len(w.entities)
		w.entities = append(w.entities, entityRec{alive: true})
	}
	id := w.nextID
	w.nextID++
	w.slots[id] = slot
	return id
}

// IsAlive reports whether the entity ID refers to a living entity.
func (w *World) IsAlive(id ID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	slot, ok := w.slots[id]
	return ok && w.entities[slot].alive
}

// DestroyEntity removes an entity and all of its components.
//
// Destroying an unknown or already destroyed entity is a no-op.
func (w *World) DestroyEntity(id ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	slot, ok := w.slots[id]
	if !ok || !w.entities[slot].alive {
		return
	}
	// Detach every component the entity owns and clear the dense slots.
	for ci, st := range w.components {
		if w.entities[slot].components&componentBit(ci) != 0 {
			st.ClearSlot(slot)
		}
	}
	w.entities[slot] = entityRec{alive: false}
	delete(w.slots, id)
	w.freeSlots = append(w.freeSlots, slot)
}

// EntityCount returns the number of living entities.
func (w *World) EntityCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	n := 0
	for i := range w.entities {
		if w.entities[i].alive {
			n++
		}
	}
	return n
}

// RegisterStorage adds a component storage to the world.
//
// Registering the same component type twice returns the existing storage so
// that callers can be sloppy without corrupting the world. Registering more
// than MaxComponentTypes types returns nil.
func (w *World) RegisterStorage(cs ComponentStorage) ComponentStorage {
	w.mu.Lock()
	defer w.mu.Unlock()
	if idx, ok := w.componentIndex[cs.ComponentID()]; ok {
		return w.components[idx]
	}
	if len(w.components) >= MaxComponentTypes {
		return nil
	}
	idx := len(w.components)
	w.components = append(w.components, cs)
	w.componentIndex[cs.ComponentID()] = idx
	return cs
}

// Storage returns the registered storage of a component type, or nil.
func (w *World) Storage(componentID string) ComponentStorage {
	w.mu.RLock()
	defer w.mu.RUnlock()
	idx, ok := w.componentIndex[componentID]
	if !ok {
		return nil
	}
	return w.components[idx]
}

// HasComponent reports whether an entity carries a component type.
func (w *World) HasComponent(id ID, componentID string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	slot, ok := w.slots[id]
	if !ok || !w.entities[slot].alive {
		return false
	}
	idx, ok := w.componentIndex[componentID]
	if !ok {
		return false
	}
	return w.entities[slot].components&componentBit(idx) != 0
}

// AddComponent attaches a component value to an entity.
//
// The value is written into the dense storage slot of the entity; if the
// entity already had the component, the value is overwritten.
func (w *World) AddComponent(id ID, cs ComponentStorage, value any) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	slot, ok := w.slots[id]
	if !ok || !w.entities[slot].alive {
		return false
	}
	idx, ok := w.componentIndex[cs.ComponentID()]
	if !ok {
		return false
	}
	cs.SetSlot(slot, value)
	cs.SetEntity(slot, id)
	w.entities[slot].components |= componentBit(idx)
	return true
}

// ComponentArray is the Array-of-Structs storage of a component type.
//
// Data is indexed by dense slot. Entities[slot] gives the owner entity ID of
// a slot, so an iteration can recover the entity without a map lookup.
type ComponentArray[T any] struct {
	// Data holds the component values, one per dense slot.
	Data []T
	// Entities maps a dense slot to the owning entity ID.
	Entities []ID
	// name is the component type name.
	name string
	// world is the owning world (set on first registration).
	world *World
}

// NewComponentArray creates an empty storage for component type T.
//
// The name defaults to the Go type name; pass an explicit name when a
// component type should be aliased.
func NewComponentArray[T any](name string) *ComponentArray[T] {
	if name == "" {
		name = reflect.TypeOf((*T)(nil)).Elem().Name()
	}
	return &ComponentArray[T]{
		Data:     make([]T, 0, 128),
		Entities: make([]ID, 0, 128),
		name:     name,
	}
}

// ComponentID implements ComponentStorage.
func (c *ComponentArray[T]) ComponentID() string { return c.name }

// Len implements ComponentStorage.
func (c *ComponentArray[T]) Len() int { return len(c.Data) }

// SlotOf returns the dense slot of an entity, or -1 when unknown.
func (c *ComponentArray[T]) SlotOf(id ID) int {
	if c.world == nil {
		return -1
	}
	c.world.mu.RLock()
	defer c.world.mu.RUnlock()
	slot, ok := c.world.slots[id]
	if !ok {
		return -1
	}
	return slot
}

// ClearSlot implements ComponentStorage.
func (c *ComponentArray[T]) ClearSlot(slot int) {
	if slot < 0 || slot >= len(c.Data) {
		return
	}
	var zero T
	c.Data[slot] = zero
	c.Entities[slot] = 0
}

// SwapSlots implements ComponentStorage.
func (c *ComponentArray[T]) SwapSlots(a, b int) {
	if a < 0 || b < 0 || a >= len(c.Data) || b >= len(c.Data) || a == b {
		return
	}
	c.Data[a], c.Data[b] = c.Data[b], c.Data[a]
	c.Entities[a], c.Entities[b] = c.Entities[b], c.Entities[a]
}

// SetEntity implements ComponentStorage: records the owner of a slot.
//
// The owner ID is what allows an iteration to recover the entity of a
// dense slot without a map lookup.
func (c *ComponentArray[T]) SetEntity(slot int, id ID) {
	for len(c.Entities) <= slot {
		c.Entities = append(c.Entities, 0)
	}
	c.Entities[slot] = id
}

// SetSlot implements ComponentStorage: writes a value into a dense slot.
func (c *ComponentArray[T]) SetSlot(slot int, value any) {
	v, ok := value.(T)
	if !ok {
		return
	}
	for len(c.Data) <= slot {
		var zero T
		c.Data = append(c.Data, zero)
		c.Entities = append(c.Entities, 0)
	}
	c.Data[slot] = v
}

// Set attaches a component value to an entity through the world's storage.
func (c *ComponentArray[T]) Set(w *World, id ID, value T) bool {
	if c.world == nil {
		c.world = w
	}
	return w.AddComponent(id, c, value)
}

// Each iterates over all entities carrying this component.
//
// The callback receives the entity ID and a pointer to the component data.
// The pointer aliases the dense storage, so the callback may modify it in
// place without allocating.
func (c *ComponentArray[T]) Each(fn func(id ID, c *T)) {
	for i := range c.Data {
		id := c.Entities[i]
		// The owner of the slot is checked against the world, so that a
		// destroyed entity is not visited.
		if c.world == nil {
			fn(id, &c.Data[i])
			continue
		}
		if !c.world.ownsSlot(id, c.ComponentID(), i) {
			continue
		}
		fn(id, &c.Data[i])
	}
}

// RegisterComponentArray registers a component storage of type T and
// returns it typed.
//
// This is the ergonomic entry point for the game code: it removes the
// need for a type assertion after every registration.
func RegisterComponentArray[T any](w *World, name string) *ComponentArray[T] {
	ca := NewComponentArray[T](name)
	registered := w.RegisterStorage(ca)
	if registered == nil {
		return nil
	}
	// The storage may already have existed; use the registered instance so
	// that the world and the storage stay in sync.
	if existing, ok := registered.(*ComponentArray[T]); ok {
		existing.world = w
		return existing
	}
	ca.world = w
	return ca
}

// ComponentArrayOf returns the registered storage of a component type T.
//
// It returns nil when the component type was never registered.
func ComponentArrayOf[T any](w *World) *ComponentArray[T] {
	storage := w.Storage(reflect.TypeOf((*T)(nil)).Elem().Name())
	if storage == nil {
		return nil
	}
	ca, _ := storage.(*ComponentArray[T])
	return ca
}

// ownsSlot reports whether an entity owns the component type in a dense
// slot.
//
// The check combines the ownership record of the storage with the
// component mask of the world, which is what keeps a destroyed entity
// from being visited by an iteration.
func (w *World) ownsSlot(id ID, componentID string, slot int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	idx, ok := w.componentIndex[componentID]
	if !ok {
		return false
	}
	owner, ok := w.slots[id]
	if !ok || !w.entities[owner].alive {
		return false
	}
	if owner != slot {
		return false
	}
	return w.entities[owner].components&componentBit(idx) != 0
}

// GetComponent returns a pointer to an entity's component of type T.
//
// It returns nil when the entity does not carry the component. The returned
// pointer points directly into the dense storage: mutating it is the
// intended, allocation-free way to update component data.
func GetComponent[T any](w *World, id ID) *T {
	storage := w.Storage(reflect.TypeOf((*T)(nil)).Elem().Name())
	if storage == nil {
		return nil
	}
	ca, ok := storage.(*ComponentArray[T])
	if !ok {
		return nil
	}
	slot := ca.SlotOf(id)
	if slot < 0 {
		return nil
	}
	// The component must actually be attached to the entity: the dense
	// storage is shared by all entities, so the slot alone does not tell
	// whether the component is present.
	if !w.hasComponentAt(id, storage.ComponentID()) {
		return nil
	}
	return &ca.Data[slot]
}

// hasComponentAt reports whether an entity owns a component type.
func (w *World) hasComponentAt(id ID, componentID string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	slot, ok := w.slots[id]
	if !ok || !w.entities[slot].alive {
		return false
	}
	idx, ok := w.componentIndex[componentID]
	if !ok {
		return false
	}
	return w.entities[slot].components&componentBit(idx) != 0
}

// String satisfies fmt.Stringer for debugging output.
func (w *World) String() string {
	return fmt.Sprintf("ecs.World{entities:%d, components:%d}", w.EntityCount(), len(w.components))
}
