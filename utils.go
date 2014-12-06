package zettajail

import "os"
import "os/exec"
import "strconv"
import "strings"

func RunCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ParseProperties(properties []string) map[string]string {
	if properties == nil {
		return nil
	}
	pmap := make(map[string]string)
	for _, property := range properties {
		isJailProperty := false
		prefix := ""

		switch property[0] {
		case '+': // "+property" is raw ZFS property
			property = property[1:]
		case '@': // "@property" is zettajail: property
			property = property[1:]
			prefix = "zettajail:"
		default: // "property" is zettajail:jail: (jail property)
			prefix = "zettajail:jail:"
			isJailProperty = true
		}

		if splut := strings.SplitN(property, "=", 2); len(splut) == 1 {
			// No "=" in string -> a flag

			// Check for negation
			isTrue := true
			if strings.HasPrefix(property, "no") {
				property = property[2:]
				isTrue = false
			} else if strings.Contains(property, ".no") {
				property = strings.Replace(property, ".no", ".", 1)
				isTrue = false
			}

			if isJailProperty {
				if isTrue {
					pmap[prefix+property] = "true"
				} else {
					pmap[prefix+property] = "false"
				}
			} else {
				if isTrue {
					pmap[prefix+property] = "on"
				} else {
					pmap[prefix+property] = "off"
				}
			}
		} else {
			if isJailProperty {
				pmap[prefix+splut[0]] = strconv.Quote(splut[1])
			} else {
				pmap[prefix+splut[0]] = splut[1]
			}
		}
	}
	return pmap
}
