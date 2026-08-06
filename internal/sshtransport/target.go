package sshtransport

// Target identifies a remote SSH endpoint by the same means a human would
// type into `ssh`. It never carries a password or private key material —
// only a path to a key file, which is passed to the system ssh binary via
// -i and never opened or copied by this package itself.
type Target struct {
	// HostOrAlias is a hostname, IP, or an alias defined in ~/.ssh/config.
	HostOrAlias string
	// Port is 0 to let ssh_config/the ssh default decide.
	Port int
	// User is "" to let ssh_config/the ssh default decide.
	User string
	// IdentityFile is "" to let ssh-agent/ssh_config decide which key to
	// offer.
	IdentityFile string
}
