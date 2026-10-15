package main

func init() {
	translations["hoststageagent"] = [3]string{"iniciando o auxiliar do servidor", "starting the server helper", "Server-Hilfsprozess wird gestartet"}
	translations["hoststageruntime"] = [3]string{"preparando a versão do servidor", "preparing the server version", "Serverversion wird vorbereitet"}
	translations["hoststagefleet"] = [3]string{"preparando os modelos locais", "preparing local vehicle models", "Lokale Fahrzeugmodelle werden vorbereitet"}
	translations["hoststageprocess"] = [3]string{"iniciando o processo do servidor", "starting the server process", "Serverprozess wird gestartet"}
	translations["hoststageloading"] = [3]string{"carregando o servidor e preparando o túnel", "loading the server and preparing the tunnel", "Server wird geladen und Tunnel vorbereitet"}
	translations["hoststagelocal"] = [3]string{"aguardando a resposta local do servidor", "waiting for the local server response", "Lokale Serverantwort wird erwartet"}
	translations["hoststageworld"] = [3]string{"aguardando o carregamento do mapa", "waiting for the map to load", "Karte wird geladen"}
	translations["hoststageclock"] = [3]string{"sincronizando o horário da empresa", "synchronizing company time", "Firmenzeit wird synchronisiert"}
	translations["hoststagetunnel"] = [3]string{"aguardando o endereço do túnel", "waiting for the tunnel address", "Tunneladresse wird erwartet"}
	translations["hoststagepublic"] = [3]string{"verificando o acesso público ao servidor", "checking public server access", "Öffentlicher Serverzugriff wird geprüft"}
	translations["hoststartuptimeout"] = [3]string{"O servidor não ficou pronto em 15 minutos. Etapa: %s. Colete os logs no Setup.", "The server was not ready after 15 minutes. Stage: %s. Collect logs in Setup.", "Server nach 15 Minuten nicht bereit. Schritt: %s. Protokolle im Setup sammeln."}
	translations["hoststartuperror"] = [3]string{"Falha na preparação do servidor; consulte o log do host", "Server preparation failed; see the host log", "Servervorbereitung fehlgeschlagen; Host-Protokoll prüfen"}
	translations["hostprocessended"] = [3]string{"O processo do servidor encerrou (%v). Colete os logs no Setup para ver a saída do servidor.", "The server process ended (%v). Collect logs in Setup to see the server output.", "Serverprozess beendet (%v). Protokolle im Setup für die Serverausgabe sammeln."}
}
