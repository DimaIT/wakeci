import Vue from "vue";
import Router from "vue-router";
import FeedView from "./views/FeedView.vue";

Vue.use(Router);

export default new Router({
    routes: [
        {
            path: "/",
            name: "feed",
            component: FeedView,
        },
        {
            path: "/tasks",
            name: "tasks",
            // route level code-splitting
            // this generates a separate chunk (about.[hash].js) for this route
            // which is lazy-loaded when the route is visited.
            component() {
                return import(/* webpackChunkName: "tasks" */ "./views/TasksView.vue");
            },
        },
    ],
});
