package database

import (
	"app/config"
	"io/ioutil"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_connectDb(t *testing.T) {

	logrus.SetOutput(ioutil.Discard)
	type args struct {
		conf *config.Databases
	}

	t.Run("Empty DB field - no connection", func(t *testing.T) {
		assert.Nil(t, DB["admin"], "Should be nil")
	})

	InitDb()

	t.Run("DB connection initialized", func(t *testing.T) {
		assert.NotNil(t, DB["admin"], "Should be nil")
	})
}
