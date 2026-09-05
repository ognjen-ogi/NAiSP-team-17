package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/engine"
)

func main() {
	e, err := engine.New("config/config.yaml")
	if err != nil {
		fmt.Println("Greska pri pokretanju engine-a:", err)
		return
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Key-Value Engine")
	fmt.Println("Komande: PUT key value, GET key, DELETE key, VALIDATE broj, EXIT")

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := strings.ToUpper(parts[0])

		switch command {
		case "PUT":
			if len(parts) < 3 {
				fmt.Println("Upotreba: PUT key value")
				continue
			}

			key := parts[1]
			value := []byte(strings.Join(parts[2:], " "))

			if err := e.Put(key, value); err != nil {
				fmt.Println("Greska:", err)
				continue
			}

			fmt.Println("OK")
			e.PrintState()

		case "GET":
			if len(parts) != 2 {
				fmt.Println("Upotreba: GET key")
				continue
			}

			value, found, err := e.Get(parts[1])
			if err != nil {
				fmt.Println("Greska:", err)
				continue
			}

			if !found {
				fmt.Println("Kljuc nije pronadjen")
				e.PrintState()
				continue
			}

			fmt.Println(string(value))
			e.PrintState()

		case "DELETE":
			if len(parts) != 2 {
				fmt.Println("Upotreba: DELETE key")
				continue
			}

			if err := e.Delete(parts[1]); err != nil {
				fmt.Println("Greska:", err)
				continue
			}

			fmt.Println("OK")
			e.PrintState()

		case "VALIDATE":
			if len(parts) != 2 {
				fmt.Println("Upotreba: VALIDATE broj")
				continue
			}

			number, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("Broj SSTable mora biti ceo broj")
				continue
			}

			result, err := e.ValidateSSTable(number)
			if err != nil {
				fmt.Println("Greska:", err)
				continue
			}

			if result.Valid {
				fmt.Println("SSTable je ispravna")
			} else {
				fmt.Println("SSTable nije ispravna")
				fmt.Println("Promenjeni zapisi:", result.ChangedRecords)
			}

			e.PrintState()

		case "EXIT":
			fmt.Println("Kraj rada")
			return

		default:
			fmt.Println("Nepoznata komanda")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Greska pri citanju ulaza:", err)
	}
}
