import http from 'k6/http';
import { sleep, check } from 'k6';

export const options = {
  vus: 1,          // number of concurrent virtual users
  duration: '30s', // test duration
};

export default function () {
  const res = http.get('http://192.168.100.69:8080/elections');

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  sleep(1);
}
