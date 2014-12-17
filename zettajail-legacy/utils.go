package jetpack

import "os"
import "os/exec"
import "strconv"
import "strings"

func RunCommand(command string, args ...string) error {
	//DONE 	cmd := exec.Command(command, args...)
	//DONE 	cmd.Stdin = os.Stdin
	//DONE 	cmd.Stdout = os.Stdout
	//DONE 	cmd.Stderr = os.Stderr
	//DONE 	return cmd.Run()
	return nil
}

func ParseProperties(properties []string) map[string]string {
	//IRRELEVANT 	if properties == nil {
	//IRRELEVANT 		return nil
	//IRRELEVANT 	}
	//IRRELEVANT 	pmap := make(map[string]string)
	//IRRELEVANT 	for _, property := range properties {
	//IRRELEVANT 		isJailProperty := false
	//IRRELEVANT 		prefix := ""
	//IRRELEVANT
	//IRRELEVANT 		switch property[0] {
	//IRRELEVANT 		case '+': // "+property" is raw ZFS property
	//IRRELEVANT 			property = property[1:]
	//IRRELEVANT 		case '@': // "@property" is jetpack: property
	//IRRELEVANT 			property = property[1:]
	//IRRELEVANT 			prefix = "jetpack:"
	//IRRELEVANT 		default: // "property" is jetpack:jail: (jail property)
	//IRRELEVANT 			prefix = "jetpack:jail:"
	//IRRELEVANT 			isJailProperty = true
	//IRRELEVANT 		}
	//IRRELEVANT
	//IRRELEVANT 		if splut := strings.SplitN(property, "=", 2); len(splut) == 1 {
	//IRRELEVANT 			// No "=" in string -> a flag
	//IRRELEVANT
	//IRRELEVANT 			// Check for negation
	//IRRELEVANT 			isTrue := true
	//IRRELEVANT 			if strings.HasPrefix(property, "no") {
	//IRRELEVANT 				property = property[2:]
	//IRRELEVANT 				isTrue = false
	//IRRELEVANT 			} else if strings.Contains(property, ".no") {
	//IRRELEVANT 				property = strings.Replace(property, ".no", ".", 1)
	//IRRELEVANT 				isTrue = false
	//IRRELEVANT 			}
	//IRRELEVANT
	//IRRELEVANT 			if isJailProperty {
	//IRRELEVANT 				if isTrue {
	//IRRELEVANT 					pmap[prefix+property] = "true"
	//IRRELEVANT 				} else {
	//IRRELEVANT 					pmap[prefix+property] = "false"
	//IRRELEVANT 				}
	//IRRELEVANT 			} else {
	//IRRELEVANT 				if isTrue {
	//IRRELEVANT 					pmap[prefix+property] = "on"
	//IRRELEVANT 				} else {
	//IRRELEVANT 					pmap[prefix+property] = "off"
	//IRRELEVANT 				}
	//IRRELEVANT 			}
	//IRRELEVANT 		} else {
	//IRRELEVANT 			if isJailProperty {
	//IRRELEVANT 				pmap[prefix+splut[0]] = strconv.Quote(splut[1])
	//IRRELEVANT 			} else {
	//IRRELEVANT 				pmap[prefix+splut[0]] = splut[1]
	//IRRELEVANT 			}
	//IRRELEVANT 		}
	//IRRELEVANT 	}
	//IRRELEVANT 	return pmap
	return nil
}
