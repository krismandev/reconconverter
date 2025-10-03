FROM fluent/fluentd:v1.17-debian-1

USER root

# Install plugin syslog + elasticsearch + lain-lain
RUN gem install fluent-plugin-remote_syslog \
    && gem install fluent-plugin-elasticsearch \
    && gem install fluent-plugin-grep \
    && gem install fluent-plugin-record-reformer \
    && gem install fluent-plugin-syslog \
    && gem install fluent-plugin-throttle

