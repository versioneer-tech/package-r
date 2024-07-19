const name: string = window.packageR.Name || "packageR";
const disableExternal: boolean = window.packageR.DisableExternal;
const disableUsedPercentage: boolean = window.packageR.DisableUsedPercentage;
const baseURL: string = window.packageR.BaseURL;
const staticURL: string = window.packageR.StaticURL;
const recaptcha: string = window.packageR.ReCaptcha;
const recaptchaKey: string = window.packageR.ReCaptchaKey;
const signup: boolean = window.packageR.Signup;
const version: string = window.packageR.Version;
const logoURL = `${staticURL}/img/logo.png`;
const noAuth: boolean = window.packageR.NoAuth;
const authMethod = window.packageR.AuthMethod;
const loginPage: boolean = window.packageR.LoginPage;
const theme: UserTheme = window.packageR.Theme;
const enableThumbs: boolean = window.packageR.EnableThumbs;
const resizePreview: boolean = window.packageR.ResizePreview;
const enableExec: boolean = window.packageR.EnableExec;
const tusSettings = window.packageR.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;

export {
  name,
  disableExternal,
  disableUsedPercentage,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
  version,
  noAuth,
  authMethod,
  loginPage,
  theme,
  enableThumbs,
  resizePreview,
  enableExec,
  tusSettings,
  origin,
  tusEndpoint,
};
