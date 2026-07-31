package notify

import (
	"fmt"
	"os/exec"
)

func Notify(text string) {
	exec.Command(
		"notify-send",
		"-a",
		"Warbler",
		"-i",
		"mail-unread",
		"Warbler",
		fmt.Sprint(text),
	).Run()
}
