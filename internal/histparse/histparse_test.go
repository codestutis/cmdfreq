package histparse

import (
	"fmt"
	"log"
	"testing"
)

func TestParseCommandEntry(t *testing.T) {
	cmd := []byte(": 12345678:2;ls")

	entry, err := parseCommandEntry(cmd)
	fmt.Println(entry)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	if entry.Command[0] != "ls" {
		log.Println("command not parsed properly")
		t.Fail()
	}

	multilineCmd := []byte(": 1234578:0;echo\\\n\"hello there\"")

	entry, err = parseCommandEntry(multilineCmd)
	fmt.Println(entry)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	if entry.Command[0] != "echo" {
		log.Println("command not parsed properly")
		t.Fail()
	}
	if entry.Command[1] != "hello there" {
		log.Println("command not parsed properly")
		t.Fail()
	}

	longCommand := []byte(": 12345678:1;cat /etc/resolv.conf | grep nameserver")
	entry, err = parseCommandEntry(longCommand)
	fmt.Println(entry)
	if entry.Command[0] != "cat" {
		log.Println("command not parsed properly")
		t.Fail()
	}
	if entry.Command[1] != "/etc/resolv.conf" {
		log.Println("command not parsed properly")
		t.Fail()
	}
	if entry.Command[2] != "|" {
		log.Println("command not parsed properly")
		t.Fail()
	}
	if entry.Command[3] != "grep" {
		log.Println("command not parsed properly")
		t.Fail()
	}
	if entry.Command[4] != "nameserver" {
		log.Println("command not parsed properly")
		t.Fail()
	}
}
