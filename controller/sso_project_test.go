package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestExchangeSSOTicketPreservesOPCAccessToken(t *testing.T) {
	for _, initialToken := range []string{"existing-opc-access-token", ""} {
		name := "existing token"
		if initialToken == "" {
			name = "missing token"
		}
		t.Run(name, func(t *testing.T) {
			db, router := setupSMSAuthTest(t)
			router.POST("/api/sso/exchange", ExchangeSSOTicket)
			user := model.User{Username: "sso-user", Password: "private-password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
			if initialToken != "" {
				user.SetAccessToken(initialToken)
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			// Repeated project authorizations must keep both OPC and earlier
			// project sessions usable, including after first-time token creation.
			wantToken := initialToken
			for i := 0; i < 2; i++ {
				ticket := t.Name()
				if err := storeSSOTicket(ticket, ssoTicketPayload{ProjectKey: "sony", UserID: user.Id, Purpose: ssoTicketPurposeLogin, ExpiresAt: time.Now().Add(time.Minute).Unix()}); err != nil {
					t.Fatal(err)
				}
				res := callSMSAuthEndpoint(t, router, "/api/sso/exchange", ssoExchangeRequest{Project: "sony", Ticket: ticket})
				if !res.Success {
					t.Fatalf("exchange failed: %s", res.Message)
				}
				var data struct {
					ID          int    `json:"id"`
					AccessToken string `json:"access_token"`
					Password    string `json:"password"`
				}
				if err := json.Unmarshal(res.Data, &data); err != nil {
					t.Fatal(err)
				}
				if wantToken == "" {
					wantToken = data.AccessToken
				}
				if data.ID != user.Id || data.AccessToken == "" || data.AccessToken != wantToken || data.Password != "" {
					t.Fatal("exchange changed the login token or returned unexpected user data")
				}
				authenticated, err := model.ValidateAccessToken("Bearer " + wantToken)
				if err != nil || authenticated == nil || authenticated.Id != user.Id {
					t.Fatalf("OPC login token no longer authenticates: %v", err)
				}
			}
		})
	}
}
