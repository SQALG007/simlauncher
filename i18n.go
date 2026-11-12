package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Langues prises en charge.
var languages = []string{"fr", "en", "pt", "es", "zh"}

var curLang atomic.Value // string

func init() { curLang.Store("en") }

// resolveLang choisit la langue : réglage explicite, sinon langue de Windows, sinon anglais.
func resolveLang(setting string) string {
	s := strings.ToLower(strings.TrimSpace(setting))
	for _, l := range languages {
		if s == l {
			return l
		}
	}
	sys := systemLang()
	for _, l := range languages {
		if strings.HasPrefix(sys, l) {
			return l
		}
	}
	return "en"
}

func setLang(l string) { curLang.Store(resolveLang(l)) }
func lang() string     { return curLang.Load().(string) }

func envLang() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return strings.ToLower(v)
		}
	}
	return ""
}

// T traduit un message du moteur.
func T(key string, args ...any) string {
	m, ok := catalog[key]
	if !ok {
		return key
	}
	f, ok := m[lang()]
	if !ok {
		f = m["en"]
	}
	if len(args) == 0 {
		return f
	}
	return fmt.Sprintf(f, args...)
}

// TE renvoie une erreur traduite.
func TE(key string, args ...any) error { return fmt.Errorf("%s", T(key, args...)) }

var catalog = map[string]map[string]string{
	// --- séquence de lancement
	"app_already":   {"fr": "%s déjà lancé", "en": "%s already running", "pt": "%s já está em execução", "es": "%s ya está en ejecución", "zh": "%s 已在运行"},
	"app_missing":   {"fr": "%s introuvable : %s", "en": "%s not found: %s", "pt": "%s não encontrado: %s", "es": "%s no encontrado: %s", "zh": "找不到 %s：%s"},
	"app_starting":  {"fr": "Démarrage de %s…", "en": "Starting %s…", "pt": "Iniciando %s…", "es": "Iniciando %s…", "zh": "正在启动 %s…"},
	"app_closing":   {"fr": "Fermeture de %s", "en": "Closing %s", "pt": "Fechando %s", "es": "Cerrando %s", "zh": "正在关闭 %s"},
	"game_unknown":  {"fr": "jeu inconnu : %s", "en": "unknown game: %s", "pt": "jogo desconhecido: %s", "es": "juego desconocido: %s", "zh": "未知游戏：%s"},
	"busy":          {"fr": "un lancement est déjà en cours (%s)", "en": "a launch is already in progress (%s)", "pt": "já há uma inicialização em andamento (%s)", "es": "ya hay un lanzamiento en curso (%s)", "zh": "已有启动正在进行（%s）"},
	"seq_start":     {"fr": "=== Lancement de %s ===", "en": "=== Launching %s ===", "pt": "=== Iniciando %s ===", "es": "=== Lanzando %s ===", "zh": "=== 正在启动 %s ==="},
	"pause":         {"fr": "Pause de %d s, le temps que les logiciels s'initialisent…", "en": "Waiting %d s for the apps to initialise…", "pt": "Aguardando %d s para os programas inicializarem…", "es": "Esperando %d s a que las aplicaciones se inicien…", "zh": "等待 %d 秒，让软件完成初始化…"},
	"game_running":  {"fr": "%s tourne déjà, pas de second lancement", "en": "%s is already running, not launching it again", "pt": "%s já está em execução, não será iniciado de novo", "es": "%s ya está en ejecución, no se lanzará otra vez", "zh": "%s 已在运行，不再重复启动"},
	"game_launch":   {"fr": "Lancement de %s", "en": "Launching %s", "pt": "Iniciando %s", "es": "Lanzando %s", "zh": "正在启动 %s"},
	"game_fail":     {"fr": "⚠ Impossible de lancer %s : %v", "en": "⚠ Could not launch %s: %v", "pt": "⚠ Não foi possível iniciar %s: %v", "es": "⚠ No se pudo lanzar %s: %v", "zh": "⚠ 无法启动 %s：%v"},
	"done_warn":     {"fr": "Terminé avec %d avertissement(s)", "en": "Done with %d warning(s)", "pt": "Concluído com %d aviso(s)", "es": "Terminado con %d advertencia(s)", "zh": "完成，有 %d 条警告"},
	"done_ok":       {"fr": "✔ %s lancé", "en": "✔ %s launched", "pt": "✔ %s iniciado", "es": "✔ %s lanzado", "zh": "✔ %s 已启动"},
	"watch":         {"fr": "Surveillance de %s (fermeture auto des logiciels activée)", "en": "Watching %s (auto-close of apps is on)", "pt": "Monitorando %s (fechamento automático dos programas ativado)", "es": "Vigilando %s (cierre automático de aplicaciones activado)", "zh": "正在监视 %s（已启用自动关闭软件）"},
	"watch_timeout": {"fr": "%s n'a pas été détecté après 5 min, surveillance abandonnée", "en": "%s was not detected after 5 min, stopped watching", "pt": "%s não foi detectado após 5 min, monitoramento encerrado", "es": "%s no se detectó tras 5 min, vigilancia cancelada", "zh": "5 分钟后仍未检测到 %s，已停止监视"},
	"game_closed":   {"fr": "%s fermé", "en": "%s closed", "pt": "%s fechado", "es": "%s cerrado", "zh": "%s 已关闭"},
	// --- validation de la configuration
	"v_app_id":      {"fr": "identifiant de logiciel invalide : %q (lettres minuscules, chiffres, - ou _)", "en": "invalid app ID: %q (lowercase letters, digits, - or _)", "pt": "ID de programa inválido: %q (letras minúsculas, números, - ou _)", "es": "ID de aplicación no válido: %q (minúsculas, dígitos, - o _)", "zh": "软件 ID 无效：%q（仅限小写字母、数字、- 或 _）"},
	"v_app_dup":     {"fr": "identifiant de logiciel en double : %s", "en": "duplicate app ID: %s", "pt": "ID de programa duplicado: %s", "es": "ID de aplicación duplicado: %s", "zh": "软件 ID 重复：%s"},
	"v_app_name":    {"fr": "le logiciel %s n'a pas de nom", "en": "app %s has no name", "pt": "o programa %s não tem nome", "es": "la aplicación %s no tiene nombre", "zh": "软件 %s 没有名称"},
	"v_app_delay":   {"fr": "délai invalide pour %s (0 à 120 s)", "en": "invalid delay for %s (0 to 120 s)", "pt": "atraso inválido para %s (0 a 120 s)", "es": "retardo no válido para %s (0 a 120 s)", "zh": "%s 的等待时间无效（0 到 120 秒）"},
	"v_game_id":     {"fr": "identifiant de jeu invalide : %q (lettres minuscules, chiffres, - ou _)", "en": "invalid game ID: %q (lowercase letters, digits, - or _)", "pt": "ID de jogo inválido: %q (letras minúsculas, números, - ou _)", "es": "ID de juego no válido: %q (minúsculas, dígitos, - o _)", "zh": "游戏 ID 无效：%q（仅限小写字母、数字、- 或 _）"},
	"v_game_dup":    {"fr": "identifiant de jeu en double : %s", "en": "duplicate game ID: %s", "pt": "ID de jogo duplicado: %s", "es": "ID de juego duplicado: %s", "zh": "游戏 ID 重复：%s"},
	"v_game_name":   {"fr": "le jeu %s n'a pas de nom", "en": "game %s has no name", "pt": "o jogo %s não tem nome", "es": "el juego %s no tiene nombre", "zh": "游戏 %s 没有名称"},
	"v_game_launch": {"fr": "%s : indique un AppID Steam ou un exécutable", "en": "%s: enter a Steam AppID or an executable", "pt": "%s: informe um AppID da Steam ou um executável", "es": "%s: indica un AppID de Steam o un ejecutable", "zh": "%s：请填写 Steam AppID 或可执行文件"},
	"v_game_appid":  {"fr": "%s : l'AppID Steam doit être un nombre", "en": "%s: the Steam AppID must be a number", "pt": "%s: o AppID da Steam deve ser um número", "es": "%s: el AppID de Steam debe ser un número", "zh": "%s：Steam AppID 必须是数字"},
	"v_game_app":    {"fr": "%s : logiciel inconnu %q", "en": "%s: unknown app %q", "pt": "%s: programa desconhecido %q", "es": "%s: aplicación desconocida %q", "zh": "%s：未知软件 %q"},
	"v_pause":       {"fr": "pause avant le jeu invalide (0 à 120 s)", "en": "invalid pause before the game (0 to 120 s)", "pt": "pausa antes do jogo inválida (0 a 120 s)", "es": "pausa antes del juego no válida (0 a 120 s)", "zh": "游戏前的等待时间无效（0 到 120 秒）"},
	"v_port":        {"fr": "port de l'interface invalide", "en": "invalid interface port", "pt": "porta da interface inválida", "es": "puerto de la interfaz no válido", "zh": "界面端口无效"},
	"v_lang":        {"fr": "langue non prise en charge : %s", "en": "unsupported language: %s", "pt": "idioma não suportado: %s", "es": "idioma no admitido: %s", "zh": "不支持的语言：%s"},
	"cfg_bad":       {"fr": "config.json illisible : %v", "en": "config.json is unreadable: %v", "pt": "config.json ilegível: %v", "es": "config.json ilegible: %v", "zh": "无法读取 config.json：%v"},
	// --- API
	"forbidden":           {"fr": "requête refusée", "en": "request refused", "pt": "solicitação recusada", "es": "solicitud rechazada", "zh": "请求被拒绝"},
	"bad_data":            {"fr": "données invalides : %v", "en": "invalid data: %v", "pt": "dados inválidos: %v", "es": "datos no válidos: %v", "zh": "数据无效：%v"},
	"not_found":           {"fr": "introuvable", "en": "not found", "pt": "não encontrado", "es": "no encontrado", "zh": "未找到"},
	"app_unknown":         {"fr": "logiciel inconnu : %s", "en": "unknown app: %s", "pt": "programa desconhecido: %s", "es": "aplicación desconocida: %s", "zh": "未知软件：%s"},
	"action_unknown":      {"fr": "action inconnue", "en": "unknown action", "pt": "ação desconhecida", "es": "acción desconocida", "zh": "未知操作"},
	"autostart_fail":      {"fr": "impossible de modifier le démarrage automatique : %v", "en": "could not change start-up setting: %v", "pt": "não foi possível alterar a inicialização automática: %v", "es": "no se pudo cambiar el inicio automático: %v", "zh": "无法更改开机自启动：%v"},
	"no_pick":             {"fr": "non disponible hors Windows", "en": "only available on Windows", "pt": "disponível apenas no Windows", "es": "solo disponible en Windows", "zh": "仅在 Windows 上可用"},
	"pick_title":          {"fr": "Choisir l'exécutable", "en": "Choose the executable", "pt": "Escolher o executável", "es": "Elegir el ejecutable", "zh": "选择可执行文件"},
	"pick_exe":            {"fr": "Programmes (*.exe)", "en": "Programs (*.exe)", "pt": "Programas (*.exe)", "es": "Programas (*.exe)", "zh": "程序 (*.exe)"},
	"pick_image":          {"fr": "Images (*.jpg, *.png, *.webp…)", "en": "Images (*.jpg, *.png, *.webp…)", "pt": "Imagens (*.jpg, *.png, *.webp…)", "es": "Imágenes (*.jpg, *.png, *.webp…)", "zh": "图片 (*.jpg, *.png, *.webp…)"},
	"pick_image_title":    {"fr": "Choisir une image", "en": "Choose an image", "pt": "Escolher uma imagem", "es": "Elegir una imagen", "zh": "选择图片"},
	"v_game_image":        {"fr": "« %s » : l'image doit être un fichier .jpg, .png, .webp, .gif ou .bmp", "en": "\"%s\": the image must be a .jpg, .png, .webp, .gif or .bmp file", "pt": "\"%s\": a imagem deve ser um arquivo .jpg, .png, .webp, .gif ou .bmp", "es": "«%s»: la imagen debe ser un archivo .jpg, .png, .webp, .gif o .bmp", "zh": "“%s”：图片必须是 .jpg、.png、.webp、.gif 或 .bmp 文件"},
	"pick_fail":           {"fr": "impossible d'ouvrir la boîte de dialogue : %v", "en": "could not open the file dialog: %v", "pt": "não foi possível abrir a caixa de diálogo: %v", "es": "no se pudo abrir el cuadro de diálogo: %v", "zh": "无法打开文件对话框：%v"},
	"upd_fail":            {"fr": "mise à jour impossible : %v", "en": "update failed: %v", "pt": "falha na atualização: %v", "es": "no se pudo actualizar: %v", "zh": "更新失败：%v"},
	"upd_none":            {"fr": "aucune mise à jour disponible", "en": "no update available", "pt": "nenhuma atualização disponível", "es": "no hay ninguna actualización disponible", "zh": "没有可用的更新"},
	"upd_bad_file":        {"fr": "le fichier téléchargé est incomplet ou altéré, mise à jour annulée", "en": "the downloaded file is incomplete or corrupted, update cancelled", "pt": "o arquivo baixado está incompleto ou corrompido, atualização cancelada", "es": "el archivo descargado está incompleto o dañado, actualización cancelada", "zh": "下载的文件不完整或已损坏，更新已取消"},
	"upd_available_n":     {"fr": "La version %s est disponible. Ouvre SimLauncher pour l'installer.", "en": "Version %s is available. Open SimLauncher to install it.", "pt": "A versão %s está disponível. Abra o SimLauncher para instalá-la.", "es": "La versión %s está disponible. Abre SimLauncher para instalarla.", "zh": "新版本 %s 已发布。打开 SimLauncher 进行安装。"},
	"upd_installing_n":    {"fr": "Installation de la version %s…", "en": "Installing version %s…", "pt": "Instalando a versão %s…", "es": "Instalando la versión %s…", "zh": "正在安装版本 %s…"},
	"end_start":           {"fr": "Fin de session…", "en": "Ending the session…", "pt": "Encerrando a sessão…", "es": "Terminando la sesión…", "zh": "正在结束会话…"},
	"end_nothing":         {"fr": "Rien à fermer.", "en": "Nothing to close.", "pt": "Nada para fechar.", "es": "Nada que cerrar.", "zh": "没有需要关闭的程序。"},
	"end_done":            {"fr": "Session terminée : %d programme(s) fermé(s).", "en": "Session ended: %d program(s) closed.", "pt": "Sessão encerrada: %d programa(s) fechado(s).", "es": "Sesión terminada: %d programa(s) cerrado(s).", "zh": "会话已结束：已关闭 %d 个程序。"},
	"tray_end":            {"fr": "Fin de session (tout fermer)", "en": "End session (close everything)", "pt": "Encerrar sessão (fechar tudo)", "es": "Terminar sesión (cerrar todo)", "zh": "结束会话（全部关闭）"},
	"steam_missing":       {"fr": "Steam est introuvable sur ce PC", "en": "Steam was not found on this PC", "pt": "O Steam não foi encontrado neste PC", "es": "No se encontró Steam en este PC", "zh": "在此电脑上未找到 Steam"},
	"steam_no_cover":      {"fr": "Steam n'a pas d'image en cache pour ce jeu (ouvre-le une fois dans la bibliothèque Steam)", "en": "Steam has no cached picture for this game (open it once in your Steam library)", "pt": "O Steam não tem imagem em cache para este jogo (abra-o uma vez na biblioteca Steam)", "es": "Steam no tiene imagen en caché para este juego (ábrelo una vez en la biblioteca de Steam)", "zh": "Steam 没有此游戏的缓存图片（请先在 Steam 库中打开一次）"},
	"steam_not_installed": {"fr": "le jeu Steam %s n'est pas installé", "en": "Steam game %s is not installed", "pt": "o jogo Steam %s não está instalado", "es": "el juego de Steam %s no está instalado", "zh": "Steam 游戏 %s 未安装"},
	"steam_no_appid":      {"fr": "« %s » n'a pas d'AppID Steam", "en": "\"%s\" has no Steam AppID", "pt": "\"%s\" não tem AppID Steam", "es": "«%s» no tiene AppID de Steam", "zh": "“%s”没有 Steam AppID"},
	"v_webhook":           {"fr": "les webhooks doivent commencer par http:// ou https://", "en": "webhooks must start with http:// or https://", "pt": "os webhooks devem começar com http:// ou https://", "es": "los webhooks deben empezar por http:// o https://", "zh": "Webhook 必须以 http:// 或 https:// 开头"},
	"webhook_empty":       {"fr": "aucune URL n'est enregistrée pour ce webhook", "en": "no URL is saved for this webhook", "pt": "nenhuma URL salva para este webhook", "es": "no hay ninguna URL guardada para este webhook", "zh": "此 Webhook 未保存 URL"},
	"pick_all":            {"fr": "Tous les fichiers", "en": "All files", "pt": "Todos os arquivos", "es": "Todos los archivos", "zh": "所有文件"},
	"shell_fail":          {"fr": "le démarrage a échoué (code %d)", "en": "start failed (code %d)", "pt": "a inicialização falhou (código %d)", "es": "el inicio falló (código %d)", "zh": "启动失败（代码 %d）"},
	// --- ligne de commande / icône
	"cli_unknown":   {"fr": "Jeu inconnu : %q\n\nOuvre SimLauncher sans argument pour voir les identifiants.", "en": "Unknown game: %q\n\nOpen SimLauncher without arguments to see the IDs.", "pt": "Jogo desconhecido: %q\n\nAbra o SimLauncher sem argumentos para ver os IDs.", "es": "Juego desconocido: %q\n\nAbre SimLauncher sin argumentos para ver los ID.", "zh": "未知游戏：%q\n\n不带参数打开 SimLauncher 可查看游戏 ID。"},
	"cli_cfg":       {"fr": "Configuration illisible :\n%v\n\nFichier : %s", "en": "Configuration unreadable:\n%v\n\nFile: %s", "pt": "Configuração ilegível:\n%v\n\nArquivo: %s", "es": "Configuración ilegible:\n%v\n\nArchivo: %s", "zh": "无法读取配置：\n%v\n\n文件：%s"},
	"warn_title":    {"fr": "SimLauncher — avertissement", "en": "SimLauncher — warning", "pt": "SimLauncher — aviso", "es": "SimLauncher — advertencia", "zh": "SimLauncher — 警告"},
	"err_title":     {"fr": "SimLauncher — erreur", "en": "SimLauncher — error", "pt": "SimLauncher — erro", "es": "SimLauncher — error", "zh": "SimLauncher — 错误"},
	"tray_open":     {"fr": "Ouvrir SimLauncher", "en": "Open SimLauncher", "pt": "Abrir o SimLauncher", "es": "Abrir SimLauncher", "zh": "打开 SimLauncher"},
	"tray_launch":   {"fr": "Lancer %s", "en": "Launch %s", "pt": "Iniciar %s", "es": "Lanzar %s", "zh": "启动 %s"},
	"tray_quit":     {"fr": "Quitter SimLauncher", "en": "Quit SimLauncher", "pt": "Sair do SimLauncher", "es": "Salir de SimLauncher", "zh": "退出 SimLauncher"},
	"tray_starting": {"fr": "Lancement de %s…", "en": "Launching %s…", "pt": "Iniciando %s…", "es": "Lanzando %s…", "zh": "正在启动 %s…"},
	"tray_warn":     {"fr": "%s lancé, avec des avertissements (voir le journal)", "en": "%s launched, with warnings (see the log)", "pt": "%s iniciado, com avisos (veja o registro)", "es": "%s lanzado, con advertencias (ver el registro)", "zh": "%s 已启动，但有警告（请查看日志）"},
}
