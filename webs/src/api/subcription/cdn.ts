import request from "@/utils/request";

export interface CDNResult {
  ip: string; delay: number; loss: number; speed: number; colo: string;
  tlsOK: boolean; link: string; name: string;
}
export interface CDNProfile {
  id: number; name: string; templateLink: string; ips: string; count: number;
  url: string; maxDelay: number; maxLoss: number; minSpeed: number; duration: number;
  groupName: string; status: string; message: string; results: CDNResult[] | null;
  startedAt?: string; finishedAt?: string; updatedAt?: string;
}
const base = "/api/v1/nodes/cdn";
export const getCDN = () => request({ url: base });
export const saveCDN = (data: CDNProfile) => request({ url: `${base}/save`, method: "post", data });
export const runCDN = (id: number) => request({ url: `${base}/${id}/run`, method: "post" });
export const cancelCDN = (id: number) => request({ url: `${base}/${id}/cancel`, method: "post" });
export const importCDN = (id: number, data: { ips: string[]; groupName: string; subscriptionID: number }) =>
  request({ url: `${base}/${id}/import`, method: "post", data });
