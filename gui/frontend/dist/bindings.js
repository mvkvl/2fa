import { Call, Events } from "/wails/runtime.js";

const SERVICE = "github.com/mvkvl/2fa/internal/service.Service";
const DESKTOP = "github.com/mvkvl/2fa/gui/internal/desktop.Desktop";

function call(fqn, ...args) {
	return Call.ByName(fqn, ...args);
}

export const api = {
	status: () => call(`${SERVICE}.Status`),
	providers: () => call(`${SERVICE}.Providers`),
	create: (provider, passphrase) => call(`${SERVICE}.Create`, provider, passphrase),
	unlock: (passphrase) => call(`${SERVICE}.Unlock`, passphrase),
	lock: () => call(`${SERVICE}.Lock`),
	listAccounts: () => call(`${SERVICE}.ListAccounts`),
	getAccount: (name) => call(`${SERVICE}.GetAccount`, name),
	saveAccount: (input) => call(`${SERVICE}.SaveAccount`, input),
	deleteAccount: (name) => call(`${SERVICE}.DeleteAccount`, name),
	setFavorite: (name, favorite) => call(`${SERVICE}.SetFavorite`, name, favorite),
	favorites: () => call(`${SERVICE}.Favorites`),
	copyCode: (name) => call(`${DESKTOP}.CopyCode`, name),
	rekey: (target, passphrase) => call(`${SERVICE}.Rekey`, target, passphrase),
	saveExport: (passphrase) => call(`${DESKTOP}.SaveExport`, passphrase),
	chooseImport: (passphrase) => call(`${DESKTOP}.ChooseImport`, passphrase),
	applyImport: (path, passphrase, overwrite) => call(`${DESKTOP}.ApplyImport`, path, passphrase, overwrite),
	refreshTray: () => call(`${DESKTOP}.RefreshTray`),
	version: () => call(`${SERVICE}.Version`),
	vaultPath: () => call(`${SERVICE}.VaultPath`),
};

export { Events };
