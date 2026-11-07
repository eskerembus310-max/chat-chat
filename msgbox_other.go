//go:build !windows

package main

import "log"

func msgBox(title, text string) { log.Println(title+":", text) }
