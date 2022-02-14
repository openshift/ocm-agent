package webhookreceiver

import log "github.com/sirupsen/logrus"

func processAMReceiver(d AMReceiverData) {
	log.WithField("AMReceiverData", d).Info("Process alert data")
}
