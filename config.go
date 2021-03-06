package sgx_server

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
)

type Configuration struct {
	// If true, the session manager will start in release mode,
	// meaning it will connect to the production version of IAS.
	Release bool

	// The subscription key for IAS API. This can be found at
	// https://api.portal.trustedservices.intel.com
	Subscription string

	// The directory that contains all the MREnclave files
	// that are acceptable for this session manager.
	Mrenclaves string

	// The directory that contains all the MRSigner files
	// that are acceptable for this session manager.
	Mrsigners string

	// Hex encoded SPID for IAS API. This can be found at
	// https://api.portal.trustedservices.intel.com
	Spid string

	// The file that contains a PEM encoded long-term ECDSA P-256
	// (SECP256R1) private key for establishing the session. The
	// public key component of this key should be built-in to the
	// client enclave.
	LongTermKey string

	// If True, then it will either prompt the user to type in the
	// password, or use LongTermKeyPassword field to decrypt the
	// long term key.
	LongTermKeyEncrypted bool

	// If LongTermKeyEncrypted is true, and this password is set
	// to an empty string, the program will prompt user for
	// input. Otherwise, LongTermKeyPassword is used as the
	// password.
	LongTermKeyPassword string

	// AllowedAdvisories maps an error during quote verification
	// to which advisories we are allowed to ignore. Current valid
	// keys are: ["CONFIGURATION_NEEDED", "GROUP_OUT_OF_DATE"].
	// Be careful to not set this too liberally.
	AllowedAdvisories map[string][]string

	// ProdID is the enclave production ID set by the entity
	// generating the enclave. It must be a 16-bit int.
	ProdID int

	// ProdSVN is the enclave security version number set by the
	// entity generating the enclave. This is used as the minimum
	// acceptable security version number, meaning any enclave
	// with higher SVN is accepted. It must be a 16-bit int.
	ProdSVN int

	// The maximum number of concurrent sessions the session
	// manager will keep alive. If MaxSessions is -1, then we
	// allow unlimited number of sessions.
	MaxSessions int

	// A session times out after Timeout minutes.
	// If there is no activity for this session within the past
	// Timeout minutes, the manager will remove the session,
	// and the client will have to reauthenticate itself.
	// If Timeout is -1, then a session will never expire.
	// except if there are more than MaxSessions sessions,
	// then the oldest ones will be removed.
	Timeout int
}

// Internal configuration used to create a session manager.
type configuration struct {
	release           bool
	subscription      string
	mrenclaves        [][MR_SIZE]byte
	mrsigners         [][MR_SIZE]byte
	spid              []byte
	longTermKey       *ecdsa.PrivateKey
	allowedAdvisories map[string][]string
	prodID            uint16
	prodSVN           uint16
	maxSessions       int
	timeout           int
}

func readMRs(dir string) [][MR_SIZE]byte {
	mrFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal("Could not read mr directory:", err)
	}

	mrs := make([][MR_SIZE]byte, len(mrFiles))
	for i, mr := range mrFiles {
		if mr.Name() == ".gitignore" {
			continue
		}

		mhex, err := ioutil.ReadFile(path.Join(dir, mr.Name()))
		if err != nil {
			log.Fatal("Could not read the MR.")
		}
		mrenclave := make([]byte, hex.DecodedLen(len(mhex)))
		l, err := hex.Decode(mrenclave, mhex)
		if err != nil {
			log.Fatal("Could not parse the hex MR.")
		}
		if l != MR_SIZE {
			log.Fatal("MR file should contain 32 bytes, but instead got", l)
		}

		copy(mrs[i][:], mrenclave[:])
	}
	return mrs
}

func readSPID(shex string) []byte {
	spid := make([]byte, hex.DecodedLen(len(shex)))
	l, err := hex.Decode(spid, []byte(shex))
	if err != nil {
		log.Fatal("Could not parse the hex spid:", err)
	} else if l != 16 {
		log.Fatal("SPID files should contain 16 bytes, but instead got", l)
	}
	return spid
}

func parseConfiguration(config *Configuration) *configuration {
	passwd := ""
	if config.LongTermKeyEncrypted {
		if config.LongTermKeyPassword != "" {
			passwd = config.LongTermKeyPassword
		} else {
			// TODO: read the password
		}
	}

	return &configuration{
		release:           config.Release,
		subscription:      config.Subscription,
		mrenclaves:        readMRs(config.Mrenclaves),
		mrsigners:         readMRs(config.Mrsigners),
		spid:              readSPID(config.Spid),
		longTermKey:       loadPrivateKey(config.LongTermKey, passwd),
		allowedAdvisories: config.AllowedAdvisories,
		prodID:            uint16(config.ProdID),
		prodSVN:           uint16(config.ProdSVN),
		maxSessions:       config.MaxSessions,
		timeout:           config.Timeout,
	}
}

// ReadConfiguration parses the configuration file, and generates the
// internal configuration to initialize the session manager.
// It will fail with log.Fatal if it could not parse the config.
func ReadConfiguration(fileName string) *Configuration {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal("Could not open configuration file:", err)
	}
	defer file.Close()

	config := &Configuration{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		log.Fatal("Could not json decode the config file:", err)
	}

	return config
}
