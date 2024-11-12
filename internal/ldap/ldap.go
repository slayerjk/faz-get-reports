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

// Search user's 'samaccountname' by it's 'displayname'
func BindAndSearchSamaccountnameByDisplayname(userAcc, ldapFqdn, ldapBasedn, bindUser, bindPass string) (string, error) {
	// forming LDAP filter
	filter := fmt.Sprintf("(&(objectClass=user)(displayname=%s)(!samaccountname=PAM-*))", userAcc)

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

	// forming LDAP search request
	searchReq := ldap.NewSearchRequest(
		ldapBasedn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"samaccountname"},
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

	// returning samaccountname
	result := conResult.Entries[0].GetAttributeValue("sAMAccountName")

	return result, nil

}
