package ecs

import "testing"

// TestEntityLifecycle verifies the creation, the component attachment and
// the destruction of an entity.
func TestEntityLifecycle(t *testing.T) {
	w := NewWorld()
	positions := RegisterComponentArray[PositionComponent](w, "PositionComponent")

	id := w.CreateEntity()
	if id == 0 {
		t.Fatal("the entity ID is zero")
	}
	if !w.IsAlive(id) {
		t.Fatal("the entity is not alive")
	}
	if w.EntityCount() != 1 {
		t.Errorf("the entity count is %d, want 1", w.EntityCount())
	}

	// --- the component is attached ---
	if !positions.Set(w, id, PositionComponent{X: 10, Y: 20}) {
		t.Fatal("the component could not be set")
	}
	if !w.HasComponent(id, "PositionComponent") {
		t.Error("the component is not registered on the entity")
	}
	pos := GetComponent[PositionComponent](w, id)
	if pos == nil {
		t.Fatal("the component cannot be read back")
	}
	if pos.X != 10 || pos.Y != 20 {
		t.Errorf("the component holds (%v, %v), want (10, 20)", pos.X, pos.Y)
	}

	// --- the component can be modified in place ---
	pos.X = 30
	if GetComponent[PositionComponent](w, id).X != 30 {
		t.Error("the in place modification was lost")
	}

	// --- the entity is destroyed ---
	w.DestroyEntity(id)
	if w.IsAlive(id) {
		t.Error("the entity is still alive after the destruction")
	}
	if w.EntityCount() != 0 {
		t.Errorf("the entity count is %d, want 0", w.EntityCount())
	}
	if GetComponent[PositionComponent](w, id) != nil {
		t.Error("the component of a destroyed entity is still readable")
	}
}

// TestComponentIsolation verifies that the dense storage does not leak a
// component from one entity to another.
func TestComponentIsolation(t *testing.T) {
	w := NewWorld()
	positions := RegisterComponentArray[PositionComponent](w, "PositionComponent")

	first := w.CreateEntity()
	positions.Set(w, first, PositionComponent{X: 100, Y: 200})

	second := w.CreateEntity()
	// The second entity has no position component.
	if GetComponent[PositionComponent](w, second) != nil {
		t.Error("an entity without the component can read it")
	}
	// The first entity still has its own value.
	pos := GetComponent[PositionComponent](w, first)
	if pos == nil || pos.X != 100 {
		t.Errorf("the value of the first entity was overwritten: %v", pos)
	}
}

// TestIteration verifies the iteration over the components.
func TestIteration(t *testing.T) {
	w := NewWorld()
	positions := RegisterComponentArray[PositionComponent](w, "PositionComponent")

	const count = 10
	ids := make([]ID, 0, count)
	for i := 0; i < count; i++ {
		id := w.CreateEntity()
		ids = append(ids, id)
		positions.Set(w, id, PositionComponent{X: float64(i)})
	}

	seen := 0
	positions.Each(func(id ID, c *PositionComponent) {
		seen++
		if c.X != float64(id-ids[0]) {
			// The IDs are sequential, so the value must match the offset.
			t.Errorf("the entity %d holds X=%v", id, c.X)
		}
	})
	if seen != count {
		t.Errorf("the iteration visited %d entities, want %d", seen, count)
	}
}

// TestSlotReuse verifies that the slots of the destroyed entities are
// reused without corrupting the components.
func TestSlotReuse(t *testing.T) {
	w := NewWorld()
	positions := RegisterComponentArray[PositionComponent](w, "PositionComponent")

	first := w.CreateEntity()
	positions.Set(w, first, PositionComponent{X: 42})
	w.DestroyEntity(first)

	second := w.CreateEntity()
	positions.Set(w, second, PositionComponent{X: 7})
	pos := GetComponent[PositionComponent](w, second)
	if pos == nil || pos.X != 7 {
		t.Errorf("the reused slot holds %v, want X=7", pos)
	}
}
