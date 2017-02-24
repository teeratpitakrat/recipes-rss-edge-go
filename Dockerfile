FROM scratch

COPY edge view.html /
EXPOSE 9090
CMD ["/edge"]
