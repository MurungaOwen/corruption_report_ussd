// Command seed populates the database with realistic demo data for local
// development and demos: ~200 officials spread across real Kenyan
// government agencies and county governments, in a realistic mix of
// verification states, so the admin dashboard and citizen verification
// page have enough data to actually look like a national system rather
// than a toy. It never runs automatically — invoke it explicitly
// (`make seed`) and never against a production database.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store/sqlite"
)

type dept struct {
	name      string
	prefix    string
	positions []string
}

var departments = []dept{
	{"National Police Service", "NPS", []string{
		"Traffic Base Commander", "OCS (Officer Commanding Station)", "Constable", "Corporal",
		"Sergeant", "Inspector", "Chief Inspector", "OCPD (Officer Commanding Police Division)",
	}},
	{"Directorate of Criminal Investigations", "DCI", []string{
		"Investigations Officer", "Detective Corporal", "Detective Sergeant", "Regional DCI Coordinator",
	}},
	{"Kenya Revenue Authority", "KRA", []string{
		"Customs & Border Control Officer", "Tax Compliance Officer", "Domestic Taxes Officer", "Station Manager",
	}},
	{"National Transport & Safety Authority", "NTSA", []string{
		"Vehicle Inspection Officer", "Licensing Officer", "Road Safety Marshal", "Enforcement Officer",
	}},
	{"Ethics & Anti-Corruption Commission", "EACC", []string{
		"Investigations Officer", "Corruption Prevention Officer", "Asset Recovery Officer", "Regional Coordinator",
	}},
	{"Kenya Immigration Department", "IMM", []string{
		"Immigration Officer", "Border Control Officer", "Passport Control Officer",
	}},
	{"Kenya Forest Service", "KFS", []string{
		"Forest Ranger", "Ecosystem Conservator", "Forest Guard",
	}},
	{"Kenya Wildlife Service", "KWS", []string{
		"Wildlife Ranger", "Warden", "Anti-Poaching Unit Officer",
	}},
	{"Ministry of Lands & Physical Planning", "LAND", []string{
		"Land Registrar", "Survey Officer", "Physical Planner", "Valuation Officer",
	}},
	{"National Environment Management Authority", "NEMA", []string{
		"Environmental Inspector", "Compliance Officer",
	}},
	{"Nairobi City County Government", "NRB", []string{
		"Revenue Collection Officer", "Building Inspector", "Public Health Officer", "Parking Enforcement Officer",
	}},
	{"Mombasa County Government", "MSA", []string{
		"Revenue Collection Officer", "Public Health Officer", "Trade Licensing Officer",
	}},
	{"Kisumu County Government", "KSM", []string{
		"Revenue Collection Officer", "Public Health Officer", "Trade Licensing Officer",
	}},
	{"Nakuru County Government", "NKR", []string{
		"Revenue Collection Officer", "Public Health Officer", "Building Inspector",
	}},
	{"Uasin Gishu County Government", "UGS", []string{
		"Revenue Collection Officer", "Agricultural Extension Officer",
	}},
	{"Judiciary of Kenya", "JUD", []string{
		"Court Clerk", "Bailiff", "Registrar",
	}},
	{"State Department for Social Protection", "SDSP", []string{
		"Social Protection Officer", "Cash Transfer Program Officer",
	}},
}

var firstNames = []string{
	"John", "Peter", "James", "David", "Daniel", "Samuel", "Joseph", "Michael", "Paul", "Stephen",
	"Grace", "Mary", "Jane", "Susan", "Catherine", "Faith", "Mercy", "Ann", "Lucy", "Esther",
	"Kiptoo", "Wanjiru", "Otieno", "Achieng", "Njeri", "Wafula", "Kamau", "Mwangi", "Cherono", "Chebet",
	"Mohamed", "Abdi", "Fatuma", "Hassan", "Amina", "Halima", "Juma", "Salim", "Zainab", "Yusuf",
}

var lastNames = []string{
	"Kiptoo", "Wanjiru", "Otieno", "Achieng", "Njeri", "Wafula", "Kamau", "Mwangi", "Cherono", "Chebet",
	"Mutua", "Kilonzo", "Odhiambo", "Onyango", "Barasa", "Simiyu", "Rotich", "Langat", "Kiplagat", "Too",
	"Abdi", "Hassan", "Juma", "Salim", "Omondi", "Owino", "Wekesa", "Nyongesa", "Gitau", "Karanja",
}

var statusWeights = []struct {
	status domain.OfficialStatus
	weight int
}{
	{domain.OfficialVerified, 78},
	{domain.OfficialUnverified, 15},
	{domain.OfficialInvestigation, 7},
}

func pickStatus(rng *rand.Rand) domain.OfficialStatus {
	total := 0
	for _, w := range statusWeights {
		total += w.weight
	}
	n := rng.Intn(total)
	for _, w := range statusWeights {
		if n < w.weight {
			return w.status
		}
		n -= w.weight
	}
	return domain.OfficialVerified
}

const targetCount = 200

func main() {
	cfg := config.Load()
	ctx := context.Background()

	s, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	rng := rand.New(rand.NewSource(42)) // fixed seed: reproducible demo data
	created, skipped := 0, 0

	for i := 0; created < targetCount && i < targetCount*3; i++ {
		d := departments[rng.Intn(len(departments))]
		position := d.positions[rng.Intn(len(d.positions))]
		name := fmt.Sprintf("%s %s", firstNames[rng.Intn(len(firstNames))], lastNames[rng.Intn(len(lastNames))])
		workID := fmt.Sprintf("%s-%05d", d.prefix, 10000+rng.Intn(89999))

		o := domain.Official{
			WorkID: workID, Name: name, Position: position, Department: d.name,
			Status: pickStatus(rng),
		}
		if err := s.CreateOfficial(ctx, &o); err != nil {
			skipped++ // work_id collision — try again with the next generated ID
			continue
		}
		created++
	}

	fmt.Printf("seed complete: %d officials created (%d collisions skipped) — db: %s\n", created, skipped, cfg.DBPath)
}
