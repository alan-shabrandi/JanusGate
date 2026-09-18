import http from "k6/http";

export const options = {
  vus: 300,
  duration: "15s",
};

export default function () {
  http.get("http://localhost:8080/api/v1/users");
}
