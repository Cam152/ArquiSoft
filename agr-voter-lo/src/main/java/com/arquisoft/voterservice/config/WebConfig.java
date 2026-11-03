package com.arquisoft.voterservice.config;

import com.arquisoft.voterservice.security.ServiceKeyInterceptor;

import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.InterceptorRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@Configuration
public class WebConfig implements WebMvcConfigurer {

    private final ServiceKeyInterceptor serviceKeyInterceptor;

    public WebConfig(ServiceKeyInterceptor serviceKeyInterceptor) {
        this.serviceKeyInterceptor = serviceKeyInterceptor;
    }

    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        registry.addInterceptor(serviceKeyInterceptor).addPathPatterns("/voters/**");
    }
}
