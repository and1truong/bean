package actionop

var names = []string{"create", "delete", "register_local_user", "transaction", "transition", "update"}

func Names() []string { return append([]string{}, names...) }

func Valid(name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}
