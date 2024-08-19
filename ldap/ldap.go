package getadusername

import (
	"fmt"

	"github.com/go-ldap/ldap"
)

func ldapConnect(ldapFqdn string) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s:389", ldapFqdn))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func LdapBindAndSearch(userAcc, ldapFqdn, ldapBasedn, bindUser, bindPass string) (*ldap.SearchResult, error) {
	// filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", userAcc)
	filter := fmt.Sprintf("(&(objectClass=user)(displayname=%s)(!samaccountname=PAM-*))", userAcc)

	conn, err := ldapConnect(ldapFqdn)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// conn.Debug = true

	errBind := conn.Bind(bindUser, bindPass)
	if errBind != nil {
		return nil, fmt.Errorf("failed to make ldap bind:\n\t%v", errBind)
	}

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

	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make ldap search:\n\t%v", err)
	}

	if len(result.Entries) > 0 {
		return result, nil
	} else {
		return nil, fmt.Errorf("failed to find any entry, empty result")
	}
}
