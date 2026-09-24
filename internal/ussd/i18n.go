package ussd

import "fmt"

// Strings holds every citizen-facing message in one language. Splitting
// this out from the flow logic in engine.go means adding a third language
// is "add a Strings value", not "touch the state machine".
type Strings struct {
	MainMenu            string
	AskWorkID           string
	OfficialNotFound    string
	AskDescription      string
	DescriptionRequired string
	AskReportID         string
	ReportNotFound      string
	InvalidChoice       string
	SessionExpired      string
	HighScrutinyNote    string
	StatusLabel         map[string]string

	OfficialVerified      func(name, position, department, verifyURL string) string
	OfficialUnverified    func(name, position, department, verifyURL string) string
	OfficialInvestigation func(name, position, department, verifyURL string) string
	ReportFiled           func(publicID, evidenceURL, code string) string
	ReportStatus          func(publicID, statusLabel string) string
}

const (
	LangEnglish = "en"
	LangSwahili = "sw"
)

// LanguagePrompt is shown before a language is chosen, so it must be
// legible to a speaker of either language — it's the one message that
// isn't looked up through Strings.
const LanguagePrompt = "CON Welcome / Karibu. Choose language / Chagua lugha:\n1. English\n2. Kiswahili"

var catalog = map[string]Strings{
	LangEnglish: {
		MainMenu: "CON Anti-Corruption & Official Verification System\n" +
			"1. Verify a Government Official\n" +
			"2. Report Corruption\n" +
			"3. Track a Report",
		AskWorkID:           "CON Enter the official's work/badge ID:",
		OfficialNotFound:    "END No official found with that ID. Be cautious — you can still report a suspected impersonation using option 2.",
		AskDescription:      "CON Describe the incident (who, what, where, when):",
		DescriptionRequired: "END A description is required. Please dial again and describe the incident.",
		AskReportID:         "CON Enter your report ID (e.g. RPT-2026-ABC123):",
		ReportNotFound:      "END No report found with that ID. Please check the ID and try again.",
		InvalidChoice:       "END Invalid choice. Please dial again and follow the menu.",
		SessionExpired:      "END Your session took too long and expired. Please dial again.",
		HighScrutinyNote:    "CAUTION: this ID has been checked unusually often recently — verify extra carefully.",
		StatusLabel: map[string]string{
			"pending":      "Pending review",
			"under_review": "Under review",
			"resolved":     "Resolved",
			"dismissed":    "Dismissed",
		},
		OfficialVerified: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END %s (%s, %s) is a VERIFIED government official.\nConfirm by photo: %s", name, position, department, verifyURL)
		},
		OfficialUnverified: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END %s (%s, %s) is NOT verified. Proceed with caution.\nDetails: %s", name, position, department, verifyURL)
		},
		OfficialInvestigation: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END CAUTION: %s (%s, %s) is currently UNDER INVESTIGATION.\nDetails: %s", name, position, department, verifyURL)
		},
		ReportFiled: func(publicID, evidenceURL, code string) string {
			return fmt.Sprintf("END Report received. Your report ID is %s — save this to track status.\nTo attach photos (optional, within 72 hrs): %s (code %s)", publicID, evidenceURL, code)
		},
		ReportStatus: func(publicID, statusLabel string) string {
			return fmt.Sprintf("END Report %s status: %s.", publicID, statusLabel)
		},
	},
	LangSwahili: {
		MainMenu: "CON Mfumo wa Kupambana na Ufisadi na Uthibitisho wa Maafisa\n" +
			"1. Thibitisha Afisa wa Serikali\n" +
			"2. Ripoti Ufisadi\n" +
			"3. Fuatilia Ripoti",
		AskWorkID:           "CON Weka kitambulisho cha kazi cha afisa:",
		OfficialNotFound:    "END Hakuna afisa aliyepatikana na kitambulisho hicho. Kuwa mwangalifu — unaweza kuripoti udanganyifu kwa chaguo la 2.",
		AskDescription:      "CON Elezea tukio (nani, nini, wapi, lini):",
		DescriptionRequired: "END Maelezo yanahitajika. Tafadhali piga tena na uelezee tukio.",
		AskReportID:         "CON Weka kitambulisho cha ripoti yako (mf. RPT-2026-ABC123):",
		ReportNotFound:      "END Hakuna ripoti iliyopatikana na kitambulisho hicho. Tafadhali angalia na ujaribu tena.",
		InvalidChoice:       "END Chaguo batili. Tafadhali piga tena na ufuate menyu.",
		SessionExpired:      "END Muda wa kikao chako umeisha. Tafadhali piga tena.",
		HighScrutinyNote:    "TAHADHARI: kitambulisho hiki kimekaguliwa mara nyingi hivi karibuni — thibitisha kwa uangalifu zaidi.",
		StatusLabel: map[string]string{
			"pending":      "Inasubiri ukaguzi",
			"under_review": "Inakaguliwa",
			"resolved":     "Imetatuliwa",
			"dismissed":    "Imekataliwa",
		},
		OfficialVerified: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END %s (%s, %s) ni afisa wa serikali ALIYETHIBITISHWA.\nThibitisha kwa picha: %s", name, position, department, verifyURL)
		},
		OfficialUnverified: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END %s (%s, %s) HAJATHIBITISHWA. Kuwa mwangalifu.\nMaelezo: %s", name, position, department, verifyURL)
		},
		OfficialInvestigation: func(name, position, department, verifyURL string) string {
			return fmt.Sprintf("END TAHADHARI: %s (%s, %s) yuko chini ya UCHUNGUZI kwa sasa.\nMaelezo: %s", name, position, department, verifyURL)
		},
		ReportFiled: func(publicID, evidenceURL, code string) string {
			return fmt.Sprintf("END Ripoti imepokelewa. Kitambulisho chako ni %s — hifadhi hii kufuatilia hali.\nKuongeza picha (hiari, ndani ya masaa 72): %s (msimbo %s)", publicID, evidenceURL, code)
		},
		ReportStatus: func(publicID, statusLabel string) string {
			return fmt.Sprintf("END Hali ya ripoti %s: %s.", publicID, statusLabel)
		},
	},
}

func stringsFor(lang string) Strings {
	if s, ok := catalog[lang]; ok {
		return s
	}
	return catalog[LangEnglish]
}
