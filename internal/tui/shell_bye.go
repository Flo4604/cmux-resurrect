package tui

import (
	"math/rand"
)

// byeMessages are shown randomly when exiting the TUI.
// Add or remove lines freely — one is picked at random each time.
var byeMessages = []string{
	"Until the next resurrection!",
	"Layouts saved. The phoenix rests!",
	"Gone but never forgotten — crex will rise again!",
	"The phoenix sleeps. Your workspaces endure!",
	"Fly safe. crex out!",
	"Ashes to ashes, tabs to tabs!",
	"The corncrake vanishes — but never for long!",
	"Your workspaces are immortal. So is crex!",
	"Rising again in 3… 2… 1…!",
	"Embers banked. Ready to ignite!",
	"This phoenix never truly exits!",
	"Workspaces secured. The bird takes flight!",
	"Nothing lost, nothing forgotten. See you soon!",
	"From the flames, crex will return!",
	"Rest now. Resurrect later!",
}

// randomBye returns a random bye message prefixed with the phoenix emoji.
func randomBye() string {
	return "🐦‍🔥 " + byeMessages[rand.Intn(len(byeMessages))]
}
