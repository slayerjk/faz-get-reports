<h1>FortiAnalyzer(FAZ) Get Reports</h1>

<p>For data program uses "data/data.json"(FAZ creds)."data/users.csv". Edit & rename "BLANK" files correspondingly.</p>

Flags are: 
    * mode('csv' or 'db'(default)), 
    * log-dir(custom log-dir; logs_get-faz-reports is default), 
    * keep-logs(number of logs to keep, 7 is default)

<h2>Description</h2>

Script create & download PDF report for AD users which pointed either in users.csv or using API of HD Naumen.

So you need to have FAZ api user creds & AD bind account to read AD tree for users.

<b>Important</b>: FAZ can't run several reports simultaneously(because we use the same datasets), so you need to wait FAZ end processing report and then start next.

<h3>Using CSV</h3>

In users.csv first column is AD displayName attribute(Full user name). So for FAZ script must use sAMAccountName - need to use LDAP to get this.

FAZ API let download only zip file(with <b>PDF</b> inside), so result(check "Results" dir in the same location as script) is zip file with name format:
```
Surname N.P._DD-MM-YYYY-T-hh-mm-ss_DD-MM-YYYY-T-hh-mm-ss
```
  * N. for Name
  * P. for Patronymic(may be blank)
  * DD-MM-YYYY-T-hh-mm-ss - datetime from start to end

<h3>Using HD Naumen API and Sqlite3 DB</h3>

Program uses Sqlite3 DB. It must be located in project root's 'data' directory and called 'data.db'.

DB is simple: 
    table 'Data' with columns:
        ID(INTEGER PRIMARY KEY), 
        Value(TEXT NOT NULL UNIQUE), 
        Posted_Date(TEXT),
        Processed(INTEGER(0(failed)/1(succeeded)/NULL(na)))
        Processed_Date(TEXT)
```
CREATE TABLE "Data" (
	"ID"	INTEGER,
	"Value"	TEXT NOT NULL UNIQUE,
	"Posted_Date"	TEXT,
	"Processed"	INTEGER,
	"Processed_Date"	TEXT,
	PRIMARY KEY("ID")
);
```

data_BLANK.db - is just empty DB with stucture described above. Rename it to data.db to use with application.

<h2>Workflow</h2>

<h3>mode 'db'</h3>
<ol>
    <li> read data file for FAZ & LDAP creds </li>
    <li>read db entries(hd naumen tasks ids) in db and get all unprocessed</li>
    <li>make api request to get all tasks data(username, startdate, enddate)
    <ol> starting report loop for each user(one in time)
        <li> search for LDAP sAMAccountName  of corresponding user in users.csv using LDAP bind user/pass </li>
        <li> get FAZ sessionid to use FAZ API using FAZ API user/pass </li>
        <li> update FAZ datasets SQL queries for corresponding user </li>
        <li> run FAZ report and wait when it will have "generated" status </li>
        <li> download and save report in Results dir(created if none) </li>
        <li>make api request to hd naumen's task, attach result to it and make it's status resolved</li>
    </ol>
</ol>

<h3>mode 'csv'</h3>
<ol>
    <li> read data file for FAZ & LDAP creds </li>
    <ol> starting report loop for each user(one in time)
        <li> search for LDAP sAMAccountName  of corresponding user in users.csv using LDAP bind user/pass </li>
        <li> get FAZ sessionid to use FAZ API using FAZ API user/pass </li>
        <li> update FAZ datasets SQL queries for corresponding user </li>
        <li> run FAZ report and wait when it will have "generated" status </li>
        <li> download and save report in Results dir(created if none) </li>
    </ol>
</ol>



