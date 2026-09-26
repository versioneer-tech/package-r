interface IUser {
  id: number;
  locale: string;
  perm: Permissions;
  singleClick: boolean;
  dateFormat: boolean;
  viewMode: ViewModeType;
}

type ViewModeType = "list" | "mosaic" | "mosaic gallery";

interface Permissions {
  create: boolean;
  delete: boolean;
  download: boolean;
  execute: boolean;
  modify: boolean;
  rename: boolean;
}

interface Sorting {
  by: string;
  asc: boolean;
}

type UserTheme = "light" | "dark" | "";
