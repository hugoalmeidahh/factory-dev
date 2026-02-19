package storage

var migrations = map[int]func(*State) error{}

func migrate(s *State) error {
	for s.SchemaVersion < CurrentSchema {
		fn, ok := migrations[s.SchemaVersion]
		if !ok {
			break
		}
		if err := fn(s); err != nil {
			return err
		}
		s.SchemaVersion++
	}
	return nil
}
