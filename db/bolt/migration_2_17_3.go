package bolt

type migration_2_17_3 struct {
	migration
}

func (d migration_2_17_3) Apply() error {
	// No-op migration for BoltDB.
	// The Inventories field is added to the Template struct and will be handled automatically.
	// Existing templates with InventoryID will have their inventories populated during FillTemplate.
	return nil
}
