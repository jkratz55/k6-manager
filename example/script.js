import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
    vus: 1,           // Number of virtual users
    duration: '10s',  // Total test duration
};

export default function () {
    http.get('https://test.k6.io');
    sleep(1);
}