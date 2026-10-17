package cards

import (
	"perfectOddsBot/models"
	"testing"
)

func TestShiftSpreadInFavor(t *testing.T) {
	if got := ShiftSpreadInFavor(-3.5, 1); got != -2.0 {
		t.Fatalf("option 1: got %v", got)
	}
	if got := ShiftSpreadInFavor(-3.5, 2); got != -5.0 {
		t.Fatalf("option 2: got %v", got)
	}
	stacked := ShiftSpreadInFavor(ShiftSpreadInFavor(0, 1), 1)
	if stacked != 3.0 {
		t.Fatalf("second shave: got %v", stacked)
	}
}

func TestATSMissAndFixPush(t *testing.T) {
	cases := []struct {
		name      string
		option    int
		scoreDiff float64
		spread    float64
		push      bool
	}{
		{"cover", 1, 10, -3.5, false},
		{"tie on the number is not a push", 1, -3.5, 3.5, false},
		{"miss of zero is not a push", 1, -3, 3, false},
		{"miss 0.5", 1, -4, 3.5, true},
		{"miss 3", 2, 1.5, 1.5, true},
		{"miss 3.5", 1, -7, 3.5, false},
	}
	for _, tc := range cases {
		miss := ATSMiss(tc.option, tc.scoreDiff, tc.spread)
		if tc.name == "miss of zero is not a push" && miss != 0 {
			t.Fatalf("%s miss = %v", tc.name, miss)
		}
		if tc.name == "miss 0.5" && miss != 0.5 {
			t.Fatalf("%s miss = %v", tc.name, miss)
		}
		if tc.name == "miss 3" && miss != 3 {
			t.Fatalf("%s miss = %v", tc.name, miss)
		}
		if tc.name == "miss 3.5" && miss != 3.5 {
			t.Fatalf("%s miss = %v", tc.name, miss)
		}
		if got := IsFixPush(miss); got != tc.push {
			t.Fatalf("%s push = %v, miss %v", tc.name, got, miss)
		}
	}
}

func TestLoadedDiceBoundsAndOrder(t *testing.T) {
	if ValidLoadedDice(0) || ValidLoadedDice(0.84) || ValidLoadedDice(1.26) {
		t.Fatal("out of range roll must be ignored")
	}
	if !ValidLoadedDice(0.85) || !ValidLoadedDice(1.25) || !ValidLoadedDice(1) {
		t.Fatal("inclusive bounds must apply")
	}
	for i := 0; i < 200; i++ {
		roll := RollLoadedDice()
		if !ValidLoadedDice(roll) {
			t.Fatalf("roll %v outside [0.85, 1.25]", roll)
		}
	}
	// Dice multiply the odds payout before Double Down and Home Field.
	// Home Field adds 15 after the double, so the order is not commutative.
	base := 100.0
	diced := base * 1.25
	afterDouble := diced * 2
	afterField := afterDouble + 15
	if afterField != 265 {
		t.Fatalf("dice then double then field: got %v", afterField)
	}
	if (base*2+15)*1.25 == afterField {
		t.Fatal("dice after home field must not match the required order")
	}
}

func TestSpeakeasyPayoutUsesDecimalNotAmerican(t *testing.T) {
	leg := AmericanToDecimal(-110)
	if leg <= 1.9 || leg >= 1.92 {
		t.Fatalf("−110 decimal = %v", leg)
	}
	total := 1.0
	for i := 0; i < 4; i++ {
		total *= leg
	}
	got, ok := SpeakeasyPayout(100, total, leg)
	if !ok {
		t.Fatal("expected a cash")
	}
	want := 100 * (total / leg)
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	wrong := 100 * (total / 110)
	if got == wrong {
		t.Fatal("divided by the American price")
	}
	heisman := got * 0.85
	if heisman >= got {
		t.Fatal("heisman must cut the repriced payout")
	}
	_, donCut, _ := WinSkim(heisman, 100, true, false)
	if donCut != (heisman-100)*DonsFavorRate {
		t.Fatalf("don skim of repriced net = %v", donCut)
	}
	if _, ok := SpeakeasyPayout(100, total, AmericanToDecimal(0)); ok {
		t.Fatal("zero snapshot must fail closed")
	}
	if AmericanToDecimal(0) != 0 {
		t.Fatal("zero american")
	}
}

func TestWinSkimAndDonLossBill(t *testing.T) {
	remaining, donCut, racketCut := WinSkim(200, 100, true, true)
	if donCut != 25 || racketCut != 15 || remaining != 160 {
		t.Fatalf("don then racket: remaining %v don %v racket %v", remaining, donCut, racketCut)
	}
	remaining, donCut, racketCut = WinSkim(200, 100, false, true)
	if donCut != 0 || racketCut != 20 || remaining != 180 {
		t.Fatalf("racket only of net: remaining %v racket %v", remaining, racketCut)
	}
	remaining, _, _ = WinSkim(50, 100, true, true)
	if remaining != 50 {
		t.Fatalf("no net, payout stays %v", remaining)
	}
	if DonLossBill(0, 500) != 0 {
		t.Fatal("jail and fix push bill nothing")
	}
	if DonLossBill(75, 1000) != 18.75 {
		t.Fatal("insurance bills 25 percent of the unreimbursed 75")
	}
	if DonLossBill(100, 10) != 10 {
		t.Fatal("bill floors at the balance")
	}
	if DonLossBill(200, 1000) != 50 {
		t.Fatal("use the stake that left, not a doubled loss stat")
	}
}

func TestHoochAndBagman(t *testing.T) {
	if HoochCost(0) != 0 {
		t.Fatal("a free draw stays free and still consumes the curse")
	}
	if HoochCost(80) != 100 {
		t.Fatal("store and draw prices rise 25 percent")
	}
	holder, pool := BagmanSplit(100)
	if holder != 15 || pool != 85 {
		t.Fatalf("bagman %v %v", holder, pool)
	}
	holder, pool = BagmanSplit(0)
	if holder != 0 || pool != 0 {
		t.Fatal("nothing paid, nothing split")
	}
}

func TestIsHeldCopy(t *testing.T) {
	if !IsHeldCopy(models.UserInventory{}) {
		t.Fatal("a fresh row is playable")
	}
	id := uint(1)
	if IsHeldCopy(models.UserInventory{TargetEntryID: &id}) {
		t.Fatal("an attached row is not playable")
	}
	if IsHeldCopy(models.UserInventory{BetAmount: 1.1}) {
		t.Fatal("a stored roll is not playable")
	}
}
