package getadusername

import (
	"fmt"

	"github.com/go-ldap/ldap"
)

// Make LDAP connection
func ldapConnect(ldapFqdn string) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s:389", ldapFqdn))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Search ONLY Enabled user's 'samaccountname' by it's 'displayname'
//
// filterName - prefix of samaAccountName to exclude from result, use "" if don't need it;
// also exclude disabled accounts (userAccountControl != 546|514);
// dnFilter - some text of full DN to INCLUDE even if account is disabled, use "" if don't need it;
func BindAndSearchSamaccountnameByDisplayname(userAcc, ldapFqdn, ldapBasedn, bindUser, bindPass, filterSama string) (string, error) {
	var result string

	// forming LDAP filter; use exclude prefix if len(filterSama) > 0
	filter := fmt.Sprintf("(&(objectClass=user)(displayname=%s))", userAcc)
	if len(filterSama) > 0 {
		filter = fmt.Sprintf("(&(objectClass=user)(displayname=%s)(!samaccountname=%s*))", userAcc, filterSama)
	}

	// make LDAP connection
	conn, err := ldapConnect(ldapFqdn)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// if debug level neede
	// conn.Debug = true

	// make LDAP bind
	errBind := conn.Bind(bindUser, bindPass)
	if errBind != nil {
		return "", fmt.Errorf("failed to make ldap bind:\n\t%v", errBind)
	}

	// forming LDAP search request for 'samaccountname' and 'useraccountcontrol'(disabled=546)
	searchReq := ldap.NewSearchRequest(
		ldapBasedn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"samaccountname", "useraccountcontrol"},
		nil,
	)

	// making LDAP search request
	conResult, err := conn.Search(searchReq)
	if err != nil {
		return "", fmt.Errorf("failed to make ldap search:\n\t%v", err)
	}

	// check if result is empty
	if len(conResult.Entries) == 0 {
		return "", fmt.Errorf("failed to find any entry, empty result")
	}

	// check if len(conResult.Entries) > 1, choose only enabled(userAccountControl != 546)
	if len(conResult.Entries) > 1 {
		for ind := range conResult.Entries {
			// pretty print
			// conResult.Entries[ind].PrettyPrint(4)

			// choose first enabled account as result
			userAccountControl := conResult.Entries[ind].GetAttributeValue("userAccountControl")

			if userAccountControl != "546" && userAccountControl != "514" {
				result = conResult.Entries[ind].GetAttributeValue("sAMAccountName")
				if len(result) == 0 {
					return "", fmt.Errorf("failed to find 'sAMAccountName' attribute value, empty result")
				}
				return result, nil
			}
		}
	}

	return "", fmt.Errorf("failed to find any ENABLED account entry")
}
