// Command licencias writes the license text of every Go module compiled into Zonda, read from the module itself, so
// a built Zonda can travel with them (the BSD, MIT and ISC licenses ask that the notice go with the program):
//
//	go run ./herramientas/licencias -salida licencias/go
//
// The list comes from `go list -deps` of the program: exactly what is compiled in, nothing that is only a test or a
// tool.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var esLicencia = regexp.MustCompile(`(?i)^(licen[cs]e|copying|unlicense|notice|patents)(\.(txt|md))?$`)

func main() {
	salida := flag.String("salida", filepath.Join("licencias", "go"), "carpeta donde escribir las licencias")
	flag.Parse()

	out, err := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}", ".").Output()
	if err != nil {
		log.Fatalf("go list: %v", err)
	}
	modulos := map[string][2]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if p := strings.Split(strings.TrimSpace(sc.Text()), "|"); len(p) == 3 {
			modulos[p[0]] = [2]string{p[1], p[2]}
		}
	}
	var nombres []string
	for m := range modulos {
		nombres = append(nombres, m)
	}
	sort.Strings(nombres)

	if err := os.MkdirAll(*salida, 0o755); err != nil {
		log.Fatal(err)
	}
	var faltan []string
	for _, m := range nombres {
		version, dir := modulos[m][0], modulos[m][1]
		entradas, _ := os.ReadDir(dir)
		var texto strings.Builder
		for _, e := range entradas {
			if e.IsDir() || !esLicencia.MatchString(e.Name()) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(&texto, "==== %s %s — %s ====\n\n%s\n\n", m, version, e.Name(), bytes.TrimSpace(b))
		}
		if texto.Len() == 0 {
			faltan = append(faltan, m)
			continue
		}
		archivo := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(m) + ".txt"
		if err := os.WriteFile(filepath.Join(*salida, archivo), []byte(texto.String()), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("licencias de %d módulos en %s\n", len(nombres)-len(faltan), *salida)
	// A module with no license file next to it cannot be redistributed blindly: the build stops and says which.
	if len(faltan) > 0 {
		log.Fatalf("estos módulos no tienen archivo de licencia: %s", strings.Join(faltan, ", "))
	}
}
