package profilevault

import "github.com/DMYTROSKORIN/wgtui/internal/domain"

// AddServer appends s to the registry. It returns an error if a server
// with the same ID already exists - callers decide what "same" means for
// display purposes, but IDs must be unique to keep server-scoped local
// metadata (profiles, peer notes) unambiguous.
func (v *Vault) AddServer(s domain.Server) error {
	for _, existing := range v.Registry.Servers {
		if existing.ID == s.ID {
			return &DuplicateServerError{ID: s.ID}
		}
	}
	v.Registry.Servers = append(v.Registry.Servers, s)
	return nil
}

// RemoveServer deletes the server with the given ID ("forget", SPEC.md
// §20.1 - this never touches the remote server itself). Returns false if
// no such server was found.
func (v *Vault) RemoveServer(id string) bool {
	for i, s := range v.Registry.Servers {
		if s.ID == id {
			v.Registry.Servers = append(v.Registry.Servers[:i], v.Registry.Servers[i+1:]...)
			return true
		}
	}
	return false
}

// GetServer returns the server with the given ID, if any.
func (v *Vault) GetServer(id string) (domain.Server, bool) {
	for _, s := range v.Registry.Servers {
		if s.ID == id {
			return s, true
		}
	}
	return domain.Server{}, false
}

// UpdateServer replaces the stored server matching s.ID. Returns false if
// no such server was found.
func (v *Vault) UpdateServer(s domain.Server) bool {
	for i, existing := range v.Registry.Servers {
		if existing.ID == s.ID {
			v.Registry.Servers[i] = s
			return true
		}
	}
	return false
}

// DuplicateServerError is returned by AddServer when the ID is already
// registered.
type DuplicateServerError struct{ ID string }

func (e *DuplicateServerError) Error() string {
	return "profilevault: server ID already registered: " + e.ID
}
