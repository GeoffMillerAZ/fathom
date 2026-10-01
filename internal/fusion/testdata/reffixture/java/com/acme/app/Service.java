package com.acme.app;

import com.acme.model.User;
import static com.acme.model.Role.of;

public class Service {
    private User owner;

    public Object role() {
        return of("admin");
    }

    public Object helper() {
        return Helper.make();
    }
}
