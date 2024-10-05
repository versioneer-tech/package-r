export interface Source {
  name: string;
  friendlyName: string;
  secretName: string;
  presignSecretName: string;
  subPath?: string;
  sets: Source[];
}
