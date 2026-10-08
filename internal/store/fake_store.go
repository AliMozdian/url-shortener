package store

// FakeStore allows injecting errors to verify HTTP error mapping
type FakeStore struct {
	records     map[string]LinkRecord
	ErrToReturn error
}

func NewFakeStore() *FakeStore {
	return &FakeStore{records: make(map[string]LinkRecord)}
}

func (f *FakeStore) Read(code string) (LinkRecord, error) {
	if f.ErrToReturn != nil {
		return LinkRecord{}, f.ErrToReturn
	}
	rec, exists := f.records[code]
	if !exists {
		return LinkRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *FakeStore) Write(record LinkRecord) error {
	if f.ErrToReturn != nil {
		return f.ErrToReturn
	}
	f.records[record.Code] = record
	return nil
}
