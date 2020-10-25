package main

import (
	"container/list"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/teirm/go_ftp/common"
)

func createTestAccount(accountName string, t *testing.T) string {
	// create an account for testing purposes
	_, err := createAccount(accountName, nil)
	if err != nil {
		t.Errorf("unable to create test account: %v", err)
	}
	return path.Join(accountRoot, accountName)
}

func TestCheckExistence(t *testing.T) {

	var tests = []struct {
		path string
		want bool
	}{
		{"/etc/hosts", true},
		{"foo-foo", false},
	}

	for _, test := range tests {
		result, err := checkExistence(test.path)
		if err != nil {
			t.Errorf("unexpected error in checkExistence")
		}

		if result != test.want {
			t.Errorf("checkExistence(%s) = %v", test.path, result)
		}
	}
}

func TestCreateAccount(t *testing.T) {
	var tests = []struct {
		account string
		resp    string
	}{
		{"test", "account created test"},
		{"test", ""},
		{"/zebra/foo", ""},
	}

	for _, test := range tests {
		respData, err := createAccount(test.account, nil)
		if err != nil && respData.Header.Info != test.resp {
			t.Errorf("createAccount(%s) = %v, %v", test.account, respData, err)
		} else if respData.Header.Info != test.resp {
			t.Errorf("createAccount(%s) = %v, %v", test.account, respData, err)
		}
	}

	for _, test := range tests {
		accountPath := path.Join(accountRoot, test.account)
		err := os.Remove(accountPath)
		if err != nil && os.IsNotExist(err) == false {
			t.Errorf("unable to cleanup %s: %v", accountPath, err)
		}
	}
}

func TestDeleteFile(t *testing.T) {
	accountName := "test"
	accountPath := createTestAccount(accountName, t)
	defer os.Remove(accountPath)

	fileName := "test_file.txt"
	filePath := path.Join(accountPath, fileName)
	fileHandle, err := os.Create(filePath)
	if err != nil {
		t.Errorf("unable to create test file: %v", err)
	}
	defer fileHandle.Close()

	resp, err := deleteFile(accountName, fileName, nil)
	if err != nil {
		t.Errorf("deleteFile(%s, %s, nil) = %v, %v, expected err == nil",
			accountName, fileName, resp, err)
	}

	resp, err = deleteFile(accountName, fileName, nil)
	if os.IsNotExist(err) == false {
		t.Errorf("repeat deleteFile(%s, %s, nil) = %v, %v, expected err == ENOENT",
			accountName, fileName, resp, err)
	}
}

func TestListFiles(t *testing.T) {
	accountName := "test"
	accountPath := createTestAccount(accountName, t)
	defer os.Remove(accountPath)

	fileMap := make(map[string]bool)
	for i := 0; i < 10; i++ {
		fh, err := ioutil.TempFile(accountPath, "test_file.*.txt")
		if err != nil {
			t.Errorf("unable to create test file: %v", err)
		}
		defer fh.Close()
		defer os.Remove(fh.Name())
		baseFileName := path.Base(fh.Name())
		fileMap[baseFileName] = true
	}

	resp, err := listFiles(accountName, nil)
	if err != nil {
		t.Errorf("unable to list files: %v", err)
	}
	dataList := resp.DataList
	for iter := dataList.Front(); iter != nil; iter = iter.Next() {
		switch x := iter.Value.(type) {
		case common.Data:
			listName := string(common.Data(x).Buffer)
			if _, ok := fileMap[listName]; !ok {
				t.Errorf("unexpected list entry: %s", listName)
			}
		default:
			t.Errorf("unknown type from DataList: %v", x)
		}
	}
}

func TestReadFile(t *testing.T) {
	accountName := "test"
	accountPath := createTestAccount(accountName, t)
	defer os.Remove(accountPath)

	var tests = []struct {
		fileName    string
		message     []byte
		expectedLen int
	}{
		{"test_file.txt", []byte("fish sticks and custard"), 1},
	}

	for _, test := range tests {
		filePath := path.Join(accountPath, test.fileName)
		err := ioutil.WriteFile(filePath, test.message, 0644)
		if err != nil {
			t.Errorf("unable to write test file: %v", err)
		}
		defer os.Remove(filePath)

		resp, err := readFile(accountName, test.fileName, nil)
		if err != nil {
			t.Errorf("unable to read test file: %v", err)
		}

		dataList := resp.DataList
		if dataList.Len() != test.expectedLen {
			t.Errorf("unexpected response size: %v, want: %d", dataList.Len(), test.expectedLen)
		}

		for iter := dataList.Front(); iter != nil; iter = iter.Next() {
			switch x := iter.Value.(type) {
			case common.Data:
				respMessage := string(common.Data(x).Buffer)
				if string(test.message) != respMessage {
					t.Errorf("unexpected message: %v", respMessage)
				}
			default:
				t.Errorf("unknown type from DataList: %v", x)
			}
		}
	}
}

func TestWriteFile(t *testing.T) {
	accountName := "test"
	accountPath := createTestAccount(accountName, t)
	defer os.Remove(accountPath)

	var tests = []struct {
		fileName string
		message  []byte
	}{
		{"test_file.txt", []byte("fish sticks and custard")},
	}

	for _, test := range tests {
		data := common.Data{len(test.message), test.message}
		dataList := list.New()
		dataList.PushBack(data)

		_, err := writeFile(accountName, test.fileName, dataList, nil)
		if err != nil {
			t.Errorf("unable to write file: %v", err)
		}

		filePath := path.Join(accountPath, test.fileName)
		defer os.Remove(filePath)

		bytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			t.Errorf("failure to read file: %v", err)
		}
		if string(bytes) != string(test.message) {
			t.Errorf("write failed: %s != %s", string(bytes), string(test.message))
		}

	}

}
