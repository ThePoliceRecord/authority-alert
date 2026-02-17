export interface IUserInfo {
  userName: string;
  sshkeyList: ISshItem[];
  sshEnabled: boolean; // Whether SSH is enabled, default false
}
interface ISshItem {
  id: string;
  name: string;
  value: string;
  addTime: number;
  latestUserdTime: string;
}
