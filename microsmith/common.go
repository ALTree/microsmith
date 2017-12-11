package microsmith

func RandString(rand int, strings []string) string {
	return strings[rand%len(strings)]
}
