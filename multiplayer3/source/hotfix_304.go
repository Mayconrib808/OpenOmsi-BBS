package main

import "strings"

const bcsReferenceTimeZone = "Europe/Berlin"

// The BCS value shown as "Time shift" is the setting the user must copy.
// BCS uses Berlin as its reference civil clock; exposing that implementation
// detail in Setup made users incorrectly calculate PC time minus game time.
func init() {
	translations["offset"] = [3]string{
		"Mudança de horário da empresa (horas)",
		"Company time shift (hours)",
		"Zeitverschiebung der Firma (Stunden)",
	}
	translations["offsethint"] = [3]string{
		"Digite exatamente o número mostrado em BCS → Empresa → Definições → Mudança de horário. Ex.: se o BCS mostrar -8 Horas, digite -8. Não calcule pela hora do seu PC ou do jogo.",
		"Enter exactly the number shown in BCS → Company → Settings → Time shift. Example: if BCS shows -8 Hours, enter -8. Do not calculate it from your PC or game clock.",
		"Trage genau die Zahl aus BBS → Firma → Einstellungen → Zeitverschiebung ein. Beispiel: Zeigt BBS -8 Stunden, gib -8 ein. Nicht aus PC- oder Spielzeit berechnen.",
	}
	translations["clockinstructions"] = [3]string{
		"No BCS, abra Empresa → Definições e copie exatamente “Mudança de horário”. A Bridge cuida do relógio de referência automaticamente.",
		"In BCS, open Company → Settings and copy the exact “Time shift” value. The Bridge handles the reference clock automatically.",
		"Öffne in BBS Firma → Einstellungen und übernimm genau die Zeitverschiebung. Die Bridge verwaltet die Referenzuhr automatisch.",
	}
	translations["clockexamplehint"] = [3]string{
		"No exemplo acima, digite -8. Use sempre o número que aparece na sua própria empresa.",
		"In the example above, enter -8. Always use the number shown by your own company.",
		"Im Beispiel oben gibst du -8 ein. Verwende immer die Zahl, die bei deiner eigenen Firma angezeigt wird.",
	}
	translations["clockplaceholder"] = [3]string{
		"Ex.: -8",
		"Example: -8",
		"Beispiel: -8",
	}
	translations["errorclock"] = [3]string{
		"A Mudança de horário é inválida. Copie exatamente o número mostrado nas definições da empresa no BCS.",
		"The Time shift is invalid. Copy exactly the number shown in the BCS company settings.",
		"Die Zeitverschiebung ist ungültig. Übernimm genau die Zahl aus den BBS-Firmeneinstellungen.",
	}

	// Keep the reference timezone internal. The hidden value preserves the
	// existing server/clock protocol while the only visible required field is
	// the BCS company Time shift.
	const startMarker = `<div class="row"><div><label for="zone">`
	const endMarker = `<small>{{t "offsethint"}}</small>`
	start := strings.Index(setupPage, startMarker)
	if start < 0 {
		return
	}
	endRel := strings.Index(setupPage[start:], endMarker)
	if endRel < 0 {
		return
	}
	end := start + endRel + len(endMarker)
	replacement := `<input type="hidden" name="zone" value="` + bcsReferenceTimeZone + `"><div><label for="offset">{{t "offset"}}</label><input id="offset" name="offset" type="number" min="-24" max="24" step="any" value="{{.Offset}}" placeholder="{{t "clockplaceholder"}}" required></div><small>{{t "offsethint"}}</small>`
	setupPage = setupPage[:start] + replacement + setupPage[end:]
}
