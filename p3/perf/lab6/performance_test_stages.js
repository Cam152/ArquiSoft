import http from 'k6/http';
import { sleep, check } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 1    }, // 1 user
    { duration: '30s', target: 50   }, // 50 users
    { duration: '30s', target: 200  }, // 200 users
    { duration: '30s', target: 500  }, // 500 users
    { duration: '30s', target: 2000 }, // 2000 users
    { duration: '30s', target: 0    }, // ramp-down
  ],
};

export default function () {
  const res = http.get('http://192.168.100.69:8080/elections');

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  sleep(1);
}
